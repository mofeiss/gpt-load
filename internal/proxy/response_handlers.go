package proxy

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"gpt-load/internal/channel"
	"gpt-load/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response, group *models.Group) (string, *models.StreamContent) {
	logrus.Debugf("【插桩日志】handleStreamingResponse开始，组: %s, 响应状态: %d", group.Name, resp.StatusCode)
	
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		responseBody, _ := ps.handleNormalResponse(c, resp, group)
		return responseBody, nil
	}

	var responseBuffer strings.Builder
	var parseBuffer bytes.Buffer
	maxSize := group.EffectiveConfig.MaxResponseBodyLogSize

	buf := make([]byte, 4*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			
			// Write to client
			if _, writeErr := c.Writer.Write(chunk); writeErr != nil {
				logUpstreamError("writing stream to client", writeErr)
				break
			}
			
			// Record to buffer for logging within size limit
			if responseBuffer.Len() < maxSize {
				remainingSpace := maxSize - responseBuffer.Len()
				if len(chunk) <= remainingSpace {
					responseBuffer.Write(chunk)
				} else {
					// Write partial chunk to stay within limit
					responseBuffer.Write(chunk[:remainingSpace])
				}
			}

			// Always record for stream parsing
			parseBuffer.Write(chunk)
			
			flusher.Flush()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			logUpstreamError("reading from upstream", err)
			break
		}
	}

	var streamContent *models.StreamContent
	// 解析流式内容（添加recover机制防止panic中断请求处理）
	if parseBuffer.Len() > 0 {
		logrus.Debugf("开始解析流式内容，数据长度: %d bytes, 渠道类型: %s", parseBuffer.Len(), group.ChannelType)
		
		func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("流式内容解析器发生panic: %v, 渠道类型: %s", r, group.ChannelType)
					// panic恢复后继续执行，确保不影响日志记录
				}
			}()
			
			parser := channel.GetStreamParser(group.ChannelType)
			if parsedContent, parseErr := parser.ParseStream(bytes.NewReader(parseBuffer.Bytes())); parseErr == nil {
				streamContent = parsedContent
				logrus.Debugf("流式内容解析成功，提取到内容 - 思维链长度: %d, 文本消息长度: %d, 工具调用长度: %d", 
					len(streamContent.ThinkingChain), len(streamContent.TextMessages), len(streamContent.ToolCalls))
			} else {
				logrus.WithError(parseErr).Warnf("流式内容解析失败，渠道类型: %s", group.ChannelType)
			}
		}()
	} else {
		logrus.Debug("解析缓冲区为空，跳过流式内容解析")
	}

	response := responseBuffer.String()
	// Add truncation indicator if response was cut off
	if responseBuffer.Len() >= maxSize {
		response += "\n[TRUNCATED: Response exceeded maximum log size]"
	}
	// 处理流式响应的gzip压缩问题
	// 检查响应是否使用了gzip压缩，如果是则解压后记录到日志
	if resp.Header.Get("Content-Encoding") == "gzip" && len(response) > 0 {
		decompressedResponse := handleGzipCompression(resp, []byte(response))
		logrus.Debugf("【插桩日志】handleStreamingResponse结束，经过gzip解压，最终响应体长度: %d", len(decompressedResponse))
		return string(decompressedResponse), streamContent
	}
	logrus.Debugf("【插桩日志】handleStreamingResponse结束，最终响应体长度: %d", len(response))
	return response, streamContent
}

func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response, group *models.Group) (string, *models.StreamContent) {
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logUpstreamError("reading response body", err)
		c.String(http.StatusInternalServerError, "Failed to read response body")
		return "", nil
	}

	// 如果上游使用了 gzip 压缩，则解压后再参与日志记录与返回
	decompressed := handleGzipCompression(resp, body)

	// Write to client（保持与上游一致，不强行重新压缩）
	if _, err := c.Writer.Write(decompressed); err != nil {
		logUpstreamError("copying response body", err)
	}

	// Return response content for logging
	maxSize := group.EffectiveConfig.MaxResponseBodyLogSize
	if len(decompressed) <= maxSize {
		return string(decompressed), nil
	} else {
		// Truncate if too large
		truncated := string(decompressed[:maxSize])
		truncated += "\n[TRUNCATED: Response exceeded maximum log size]"
		return truncated, nil
	}
}
