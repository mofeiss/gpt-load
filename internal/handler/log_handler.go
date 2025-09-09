package handler

import (
	"fmt"
	app_errors "gpt-load/internal/errors"
	"gpt-load/internal/models"
	"gpt-load/internal/response"
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// LogResponse defines the structure for log entries in the API response
type LogResponse struct {
	models.RequestLog
}

// GetLogs handles fetching request logs with filtering and pagination.
func (s *Server) GetLogs(c *gin.Context) {
	query := s.LogService.GetLogsQuery(c)

	var logs []models.RequestLog
	query = query.Order("timestamp desc")
	pagination, err := response.Paginate(c, query, &logs)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	pagination.Items = logs
	response.Success(c, pagination)
}

// ExportLogs handles exporting filtered log keys to a CSV file.
func (s *Server) ExportLogs(c *gin.Context) {
	filename := fmt.Sprintf("log_keys_export_%s.csv", time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("Content-Type", "text/csv; charset=utf-8")

	// Stream the response
	err := s.LogService.StreamLogKeysToCSV(c, c.Writer)
	if err != nil {
		log.Printf("Failed to stream log keys to CSV: %v", err)
		c.JSON(500, gin.H{"error": "Failed to export logs"})
		return
	}
}

// DeleteLogsRequest defines the request structure for deleting logs by IDs
type DeleteLogsRequest struct {
	LogIds []string `json:"log_ids" binding:"required"`
}

// DeleteLogs handles deleting logs by their IDs.
func (s *Server) DeleteLogs(c *gin.Context) {
	var req DeleteLogsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	if len(req.LogIds) == 0 {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "日志ID列表不能为空"))
		return
	}

	deletedCount, err := s.LogService.DeleteLogsByIds(req.LogIds)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, gin.H{
		"deleted_count": deletedCount,
		"message":       fmt.Sprintf("成功删除 %d 条日志", deletedCount),
	})
}
