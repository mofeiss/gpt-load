package proxy

import (
	"io"
	"net/http"
	"strings"

	"gpt-load/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func (ps *ProxyServer) handleStreamingResponse(c *gin.Context, resp *http.Response, group *models.Group) string {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		logrus.Error("Streaming unsupported by the writer, falling back to normal response")
		return ps.handleNormalResponse(c, resp, group)
	}

	var responseBuffer strings.Builder
	maxSize := group.EffectiveConfig.MaxResponseBodyLogSize
	shouldLog := group.EffectiveConfig.EnableResponseBodyLogging

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
			
			// Record to buffer for logging if enabled and within size limit
			if shouldLog && responseBuffer.Len() < maxSize {
				remainingSpace := maxSize - responseBuffer.Len()
				if len(chunk) <= remainingSpace {
					responseBuffer.Write(chunk)
				} else {
					// Write partial chunk to stay within limit
					responseBuffer.Write(chunk[:remainingSpace])
				}
			}
			
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

	if shouldLog {
		response := responseBuffer.String()
		// Add truncation indicator if response was cut off
		if responseBuffer.Len() >= maxSize {
			response += "\n[TRUNCATED: Response exceeded maximum log size]"
		}
		// 处理流式响应的gzip压缩问题
		// 检查响应是否使用了gzip压缩，如果是则解压后记录到日志
		if resp.Header.Get("Content-Encoding") == "gzip" && len(response) > 0 {
			decompressedResponse := handleGzipCompression(resp, []byte(response))
			return string(decompressedResponse)
		}
		return response
	}

	return ""
}

func (ps *ProxyServer) handleNormalResponse(c *gin.Context, resp *http.Response, group *models.Group) string {
	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logUpstreamError("reading response body", err)
		c.String(http.StatusInternalServerError, "Failed to read response body")
		return ""
	}

	// 如果上游使用了 gzip 压缩，则解压后再参与日志记录与返回
	decompressed := handleGzipCompression(resp, body)

	// Write to client（保持与上游一致，不强行重新压缩）
	if _, err := c.Writer.Write(decompressed); err != nil {
		logUpstreamError("copying response body", err)
	}

	// Return response content for logging if enabled
	if group.EffectiveConfig.EnableResponseBodyLogging {
		maxSize := group.EffectiveConfig.MaxResponseBodyLogSize
		if len(decompressed) <= maxSize {
			return string(decompressed)
		} else {
			// Truncate if too large
			truncated := string(decompressed[:maxSize])
			truncated += "\n[TRUNCATED: Response exceeded maximum log size]"
			return truncated
		}
	}

	return ""
}
