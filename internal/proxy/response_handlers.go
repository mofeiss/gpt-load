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

	// Write to client
	if _, err := c.Writer.Write(body); err != nil {
		logUpstreamError("copying response body", err)
	}

	// Return response content for logging if enabled
	if group.EffectiveConfig.EnableResponseBodyLogging {
		maxSize := group.EffectiveConfig.MaxResponseBodyLogSize
		if len(body) <= maxSize {
			return string(body)
		} else {
			// Truncate if too large
			truncated := string(body[:maxSize])
			truncated += "\n[TRUNCATED: Response exceeded maximum log size]"
			return truncated
		}
	}

	return ""
}
