// Package proxy provides high-performance OpenAI multi-key proxy server
package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gpt-load/internal/channel"
	"gpt-load/internal/config"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/keypool"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"gpt-load/internal/services"
	"gpt-load/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// ProxyServer represents the proxy server
type ProxyServer struct {
	keyProvider       *keypool.KeyProvider
	groupManager      *services.GroupManager
	settingsManager   *config.SystemSettingsManager
	channelFactory    *channel.Factory
	requestLogService *services.RequestLogService
}

// NewProxyServer creates a new proxy server
func NewProxyServer(
	keyProvider *keypool.KeyProvider,
	groupManager *services.GroupManager,
	settingsManager *config.SystemSettingsManager,
	channelFactory *channel.Factory,
	requestLogService *services.RequestLogService,
) (*ProxyServer, error) {
	return &ProxyServer{
		keyProvider:       keyProvider,
		groupManager:      groupManager,
		settingsManager:   settingsManager,
		channelFactory:    channelFactory,
		requestLogService: requestLogService,
	}, nil
}

// HandleProxy is the main entry point for proxy requests, refactored based on the stable .bak logic.
func (ps *ProxyServer) HandleProxy(c *gin.Context) {
	startTime := time.Now()
	groupName := c.Param("group_name")
	path := c.Param("path")

	// 解析路径，判断是否为单密钥请求
	var specificKeyID uint
	var actualPath string
	isSpecificKey := false

	if strings.HasPrefix(path, "/id_") {
		// 提取 id_ 后面的数字部分
		parts := strings.SplitN(path[4:], "/", 2) // 去掉 "/id_" 后分割
		if len(parts) >= 1 {
			if keyID, err := strconv.ParseUint(parts[0], 10, 32); err == nil {
				specificKeyID = uint(keyID)
				isSpecificKey = true
				if len(parts) > 1 {
					actualPath = "/" + parts[1] // 重新构建实际的API路径
				} else {
					actualPath = "/"
				}
			}
		}
	}

	if !isSpecificKey {
		actualPath = path
	}

	// 临时修改 gin.Context 的路径参数，以便后续处理使用正确的路径
	c.Request.URL.Path = "/proxy/" + groupName + actualPath

	group, err := ps.groupManager.GetGroupByName(groupName)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	channelHandler, err := ps.channelFactory.GetChannel(group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get channel for group '%s': %v", groupName, err)))
		return
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		logrus.Errorf("Failed to read request body: %v", err)
		response.Error(c, app_errors.NewAPIError(app_errors.ErrBadRequest, "Failed to read request body"))
		return
	}
	c.Request.Body.Close()

	finalBodyBytes, err := ps.applyParamOverrides(bodyBytes, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to apply parameter overrides: %v", err)))
		return
	}

	isStream := channelHandler.IsStreamRequest(c, bodyBytes)

	ps.executeRequestWithRetry(c, channelHandler, group, finalBodyBytes, isStream, startTime, 0, isSpecificKey, specificKeyID)
}

// executeRequestWithRetry is the core recursive function for handling requests and retries.
func (ps *ProxyServer) executeRequestWithRetry(
	c *gin.Context,
	channelHandler channel.ChannelProxy,
	group *models.Group,
	bodyBytes []byte,
	isStream bool,
	startTime time.Time,
	retryCount int,
	isSpecificKey bool,
	specificKeyID uint,
) {
	cfg := group.EffectiveConfig

	var apiKey *models.APIKey
	var err error

	if isSpecificKey {
		// 使用指定的密钥ID
		apiKey, err = ps.keyProvider.SelectKeyByID(group.ID, specificKeyID)
		if err != nil {
			logrus.Errorf("Failed to get specific key %d for group %s: %v", specificKeyID, group.Name, err)
			if apiErr, ok := err.(*app_errors.APIError); ok {
				response.Error(c, apiErr)
			} else {
				response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to get key: %v", err)))
			}
			ps.logRequest(c, group, nil, startTime, http.StatusBadRequest, err, isStream, "", channelHandler, bodyBytes, models.RequestTypeFinal, "")
			return
		}
	} else {
		// 使用密钥池轮询
		apiKey, err = ps.keyProvider.SelectKey(group.ID)
		if err != nil {
			logrus.Errorf("Failed to select a key for group %s on attempt %d: %v", group.Name, retryCount+1, err)
			response.Error(c, app_errors.NewAPIError(app_errors.ErrNoKeysAvailable, err.Error()))
			ps.logRequest(c, group, nil, startTime, http.StatusServiceUnavailable, err, isStream, "", channelHandler, bodyBytes, models.RequestTypeFinal, "")
			return
		}
	}

	upstreamURL, err := channelHandler.BuildUpstreamURL(c.Request.URL, group)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInternalServer, fmt.Sprintf("Failed to build upstream URL: %v", err)))
		return
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if isStream {
		ctx, cancel = context.WithCancel(c.Request.Context())
	} else {
		timeout := time.Duration(cfg.RequestTimeout) * time.Second
		ctx, cancel = context.WithTimeout(c.Request.Context(), timeout)
	}
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, c.Request.Method, upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		logrus.Errorf("Failed to create upstream request: %v", err)
		response.Error(c, app_errors.ErrInternalServer)
		return
	}
	req.ContentLength = int64(len(bodyBytes))

	req.Header = c.Request.Header.Clone()

	// Clean up client auth key
	req.Header.Del("Authorization")
	req.Header.Del("X-Api-Key")
	req.Header.Del("X-Goog-Api-Key")

	channelHandler.ModifyRequest(req, apiKey, group)

	// Apply custom header rules after channel-specific modifications
	if len(group.HeaderRuleList) > 0 {
		headerCtx := utils.NewHeaderVariableContextFromGin(c, group, apiKey)
		utils.ApplyHeaderRules(req, group.HeaderRuleList, headerCtx)
	}

	var client *http.Client
	if isStream {
		client = channelHandler.GetStreamClient()
		req.Header.Set("X-Accel-Buffering", "no")
	} else {
		client = channelHandler.GetHTTPClient()
	}

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	// Unified error handling for retries. Exclude 404 from being a retryable error.
	if err != nil || (resp != nil && resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound) {
		if err != nil && app_errors.IsIgnorableError(err) {
			logrus.Debugf("Client-side ignorable error for key %s, aborting retries: %v", utils.MaskAPIKey(apiKey.KeyValue), err)
			ps.logRequest(c, group, apiKey, startTime, 499, err, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal, "")
			return
		}

		var statusCode int
		var errorMessage string
		var parsedError string

		if err != nil {
			statusCode = 500
			errorMessage = err.Error()
			parsedError = errorMessage
			logrus.Debugf("Request failed (attempt %d/%d) for key %s: %v", retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), err)
		} else {
			// HTTP-level error (status >= 400)
			statusCode = resp.StatusCode
			errorBody, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				logrus.Errorf("Failed to read error body: %v", readErr)
				errorBody = []byte("Failed to read error body")
			}

			errorBody = handleGzipCompression(resp, errorBody)
			errorMessage = string(errorBody)
			parsedError = app_errors.ParseUpstreamError(errorBody)
			logrus.Debugf("Request failed with status %d (attempt %d/%d) for key %s. Parsed Error: %s", statusCode, retryCount+1, cfg.MaxRetries, utils.MaskAPIKey(apiKey.KeyValue), parsedError)
		}

		// 使用解析后的错误信息更新密钥状态
		ps.keyProvider.UpdateStatus(apiKey, group, false, parsedError)

		// 单密钥模式下不进行重试，直接返回错误
		if isSpecificKey {
			var errorJSON map[string]any
			if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
				c.JSON(statusCode, errorJSON)
			} else {
				response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
			}
			ps.logRequest(c, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal, "")
			return
		}

		// 判断是否为最后一次尝试
		isLastAttempt := retryCount >= cfg.MaxRetries
		requestType := models.RequestTypeRetry
		if isLastAttempt {
			requestType = models.RequestTypeFinal
		}

		ps.logRequest(c, group, apiKey, startTime, statusCode, errors.New(parsedError), isStream, upstreamURL, channelHandler, bodyBytes, requestType, "")

		// 如果是最后一次尝试，直接返回错误，不再递归
		if isLastAttempt {
			var errorJSON map[string]any
			if err := json.Unmarshal([]byte(errorMessage), &errorJSON); err == nil {
				c.JSON(statusCode, errorJSON)
			} else {
				response.Error(c, app_errors.NewAPIErrorWithUpstream(statusCode, "UPSTREAM_ERROR", errorMessage))
			}
			return
		}

		// 添加重试间隔等待
		if cfg.RetryIntervalMs > 0 {
			logrus.Debugf("Waiting %d ms before retry attempt %d", cfg.RetryIntervalMs, retryCount+2)
			time.Sleep(time.Duration(cfg.RetryIntervalMs) * time.Millisecond)
		}

		ps.executeRequestWithRetry(c, channelHandler, group, bodyBytes, isStream, startTime, retryCount+1, isSpecificKey, specificKeyID)
		return
	}

	// ps.keyProvider.UpdateStatus(apiKey, group, true) // 请求成功不再重置成功次数，减少IO消耗
	logrus.Debugf("Request for group %s succeeded on attempt %d with key %s", group.Name, retryCount+1, utils.MaskAPIKey(apiKey.KeyValue))

	for key, values := range resp.Header {
		for _, value := range values {
			c.Header(key, value)
		}
	}
	c.Status(resp.StatusCode)

	logrus.Debugf("【插桩日志】开始处理响应体，流式请求: %v, 组: %s", isStream, group.Name)
	
	var responseBody string
	var streamContent *models.StreamContent
	if isStream {
		logrus.Debugf("【插桩日志】调用handleStreamingResponse处理流式响应")
		responseBody, streamContent = ps.handleStreamingResponse(c, resp, group)
		logrus.Debugf("【插桩日志】handleStreamingResponse完成，响应体长度: %d", len(responseBody))
	} else {
		logrus.Debugf("【插桩日志】调用handleNormalResponse处理普通响应")
		responseBody, _ = ps.handleNormalResponse(c, resp, group)
		logrus.Debugf("【插桩日志】handleNormalResponse完成，响应体长度: %d", len(responseBody))
	}

	logrus.Debugf("【插桩日志】准备记录请求日志，响应体长度: %d, 流式内容: %v", len(responseBody), streamContent != nil)
	ps.logRequestWithStreamContent(c, group, apiKey, startTime, resp.StatusCode, nil, isStream, upstreamURL, channelHandler, bodyBytes, models.RequestTypeFinal, responseBody, streamContent)
	logrus.Debugf("【插桩日志】请求日志记录完成")
}

// logRequest is a helper function to create and record a request log.
func (ps *ProxyServer) logRequest(
	c *gin.Context,
	group *models.Group,
	apiKey *models.APIKey,
	startTime time.Time,
	statusCode int,
	finalError error,
	isStream bool,
	upstreamAddr string,
	channelHandler channel.ChannelProxy,
	bodyBytes []byte,
	requestType string,
	responseBody string,
) {
	if ps.requestLogService == nil {
		return
	}

	var requestBodyToLog, userAgent string

	requestBodyToLog = utils.TruncateString(string(bodyBytes), group.EffectiveConfig.MaxRequestBodyLogSize)
	userAgent = c.Request.UserAgent()

	duration := time.Since(startTime).Milliseconds()

	logEntry := &models.RequestLog{
		GroupID:      group.ID,
		GroupName:    group.Name,
		IsSuccess:    finalError == nil && statusCode < 400,
		SourceIP:     c.ClientIP(),
		StatusCode:   statusCode,
		RequestPath:  utils.TruncateString(c.Request.URL.String(), 500),
		Duration:     duration,
		UserAgent:    userAgent,
		RequestType:  requestType,
		IsStream:     isStream,
		UpstreamAddr: utils.TruncateString(upstreamAddr, 500),
		RequestBody:  requestBodyToLog,
		ResponseBody: utils.TruncateString(responseBody, 65000),
	}

	if channelHandler != nil && bodyBytes != nil {
		logEntry.Model = channelHandler.ExtractModel(c, bodyBytes)
	}

	if apiKey != nil {
		logEntry.KeyValue = apiKey.KeyValue
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	if err := ps.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("Failed to record request log: %v", err)
	}
}

// logRequestWithStreamContent is a helper function to create and record a request log with stream content.
func (ps *ProxyServer) logRequestWithStreamContent(
	c *gin.Context,
	group *models.Group,
	apiKey *models.APIKey,
	startTime time.Time,
	statusCode int,
	finalError error,
	isStream bool,
	upstreamAddr string,
	channelHandler channel.ChannelProxy,
	bodyBytes []byte,
	requestType string,
	responseBody string,
	streamContent *models.StreamContent,
) {
	logrus.Debugf("【插桩日志】logRequestWithStreamContent开始，组: %s, 状态码: %d, 流式: %v, 响应体长度: %d", group.Name, statusCode, isStream, len(responseBody))
	
	if ps.requestLogService == nil {
		logrus.Error("【插桩日志】requestLogService为nil，无法记录日志！")
		return
	}
	
	logrus.Debugf("【插桩日志】requestLogService可用，开始构建日志条目")

	var requestBodyToLog, userAgent string

	requestBodyToLog = utils.TruncateString(string(bodyBytes), group.EffectiveConfig.MaxRequestBodyLogSize)
	userAgent = c.Request.UserAgent()

	duration := time.Since(startTime).Milliseconds()

	logEntry := &models.RequestLog{
		GroupID:       group.ID,
		GroupName:     group.Name,
		IsSuccess:     finalError == nil && statusCode < 400,
		SourceIP:      c.ClientIP(),
		StatusCode:    statusCode,
		RequestPath:   utils.TruncateString(c.Request.URL.String(), 500),
		Duration:      duration,
		UserAgent:     userAgent,
		RequestType:   requestType,
		IsStream:      isStream,
		UpstreamAddr:  utils.TruncateString(upstreamAddr, 500),
		RequestBody:   requestBodyToLog,
		ResponseBody:  utils.TruncateString(responseBody, 65000),
		StreamContent: streamContent,
	}

	if channelHandler != nil && bodyBytes != nil {
		logEntry.Model = channelHandler.ExtractModel(c, bodyBytes)
	}

	if apiKey != nil {
		logEntry.KeyValue = apiKey.KeyValue
	}

	if finalError != nil {
		logEntry.ErrorMessage = finalError.Error()
	}

	logrus.Debugf("【插桩日志】准备调用requestLogService.Record，日志条目ID: %s", logEntry.ID)
	if err := ps.requestLogService.Record(logEntry); err != nil {
		logrus.Errorf("【插桩日志】记录请求日志失败: %v", err)
	} else {
		logrus.Debugf("【插桩日志】请求日志成功记录到数据库，ID: %s", logEntry.ID)
	}
}
