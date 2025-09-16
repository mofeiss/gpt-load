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

// ClearLogs handles clearing all logs.
func (s *Server) ClearLogs(c *gin.Context) {
	deletedCount, err := s.LogService.ClearAllLogs()
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, gin.H{
		"deleted_count": deletedCount,
		"message":       fmt.Sprintf("成功清空 %d 条日志", deletedCount),
	})
}

// CleanupDetailedContentRequest defines the request structure for cleaning up detailed content
type CleanupDetailedContentRequest struct {
	MaxSizeKB *int `json:"max_size_kb,omitempty"` // 如果指定，只清理超过此大小的记录
}

// CleanupDetailedContent 清理详细内容但保留记录摘要
func (s *Server) CleanupDetailedContent(c *gin.Context) {
	var req CleanupDetailedContentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	var cleanedCount int64
	var err error

	if req.MaxSizeKB != nil && *req.MaxSizeKB > 0 {
		// 只清理超过指定大小的记录
		cleanedCount, err = s.LogService.CleanupLargeRecords(*req.MaxSizeKB)
	} else {
		// 清理所有详细内容
		cleanedCount, err = s.LogService.CleanupDetailedContent()
	}

	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, gin.H{
		"cleaned_count": cleanedCount,
		"message":       fmt.Sprintf("成功清理 %d 条日志的详细内容", cleanedCount),
	})
}

// CleanupByTimeRangeRequest defines the request structure for time range cleanup
type CleanupByTimeRangeRequest struct {
	StartTime   string `json:"start_time" binding:"required"`   // RFC3339格式
	EndTime     string `json:"end_time" binding:"required"`     // RFC3339格式
	OnlyDetails bool   `json:"only_details"`                    // 是否只清理详细内容
}

// CleanupByTimeRange 按时间范围清理日志
func (s *Server) CleanupByTimeRange(c *gin.Context) {
	var req CleanupByTimeRangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "开始时间格式不正确"))
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "结束时间格式不正确"))
		return
	}

	if startTime.After(endTime) {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrValidation, "开始时间不能晚于结束时间"))
		return
	}

	cleanedCount, err := s.LogService.CleanupByTimeRange(startTime, endTime, req.OnlyDetails)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	action := "清理"
	if !req.OnlyDetails {
		action = "删除"
	}

	response.Success(c, gin.H{
		"cleaned_count": cleanedCount,
		"message":       fmt.Sprintf("成功%s %d 条日志", action, cleanedCount),
	})
}

// CleanupByGroupRequest defines the request structure for group cleanup
type CleanupByGroupRequest struct {
	GroupName   string `json:"group_name" binding:"required"`
	OnlyDetails bool   `json:"only_details"`
}

// CleanupByGroup 按分组清理日志
func (s *Server) CleanupByGroup(c *gin.Context) {
	var req CleanupByGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, app_errors.NewAPIError(app_errors.ErrInvalidJSON, err.Error()))
		return
	}

	cleanedCount, err := s.LogService.CleanupByGroup(req.GroupName, req.OnlyDetails)
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	action := "清理详细内容"
	if !req.OnlyDetails {
		action = "删除"
	}

	response.Success(c, gin.H{
		"cleaned_count": cleanedCount,
		"message":       fmt.Sprintf("成功%s分组 %s 的 %d 条日志", action, req.GroupName, cleanedCount),
	})
}

// GetDatabaseStats 获取数据库统计信息
func (s *Server) GetDatabaseStats(c *gin.Context) {
	stats, err := s.LogService.GetDatabaseSize()
	if err != nil {
		response.Error(c, app_errors.ParseDBError(err))
		return
	}

	response.Success(c, stats)
}
