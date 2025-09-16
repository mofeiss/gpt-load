package services

import (
	"encoding/csv"
	"fmt"
	"gpt-load/internal/models"
	"io"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ExportableLogKey defines the structure for the data to be exported to CSV.
type ExportableLogKey struct {
	KeyValue   string `gorm:"column:key_value"`
	GroupName  string `gorm:"column:group_name"`
	StatusCode int    `gorm:"column:status_code"`
}

// LogService provides services related to request logs.
type LogService struct {
	DB *gorm.DB
}

// NewLogService creates a new LogService.
func NewLogService(db *gorm.DB) *LogService {
	return &LogService{DB: db}
}

// logFiltersScope returns a GORM scope function that applies filters from the Gin context.
func logFiltersScope(c *gin.Context) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if groupName := c.Query("group_name"); groupName != "" {
			db = db.Where("group_name LIKE ?", "%"+groupName+"%")
		}
		if keyValue := c.Query("key_value"); keyValue != "" {
			db = db.Where("key_value LIKE ?", "%"+keyValue+"%")
		}
		if model := c.Query("model"); model != "" {
			db = db.Where("model LIKE ?", "%"+model+"%")
		}
		if isSuccessStr := c.Query("is_success"); isSuccessStr != "" {
			if isSuccess, err := strconv.ParseBool(isSuccessStr); err == nil {
				db = db.Where("is_success = ?", isSuccess)
			}
		}
		if requestType := c.Query("request_type"); requestType != "" {
			db = db.Where("request_type = ?", requestType)
		}
		if statusCodeStr := c.Query("status_code"); statusCodeStr != "" {
			if statusCode, err := strconv.Atoi(statusCodeStr); err == nil {
				db = db.Where("status_code = ?", statusCode)
			}
		}
		if sourceIP := c.Query("source_ip"); sourceIP != "" {
			db = db.Where("source_ip = ?", sourceIP)
		}
		if errorContains := c.Query("error_contains"); errorContains != "" {
			db = db.Where("error_message LIKE ?", "%"+errorContains+"%")
		}
		if startTimeStr := c.Query("start_time"); startTimeStr != "" {
			if startTime, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
				db = db.Where("timestamp >= ?", startTime)
			}
		}
		if endTimeStr := c.Query("end_time"); endTimeStr != "" {
			if endTime, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
				db = db.Where("timestamp <= ?", endTime)
			}
		}
		return db
	}
}

// GetLogsQuery returns a GORM query for fetching logs with filters.
func (s *LogService) GetLogsQuery(c *gin.Context) *gorm.DB {
	return s.DB.Model(&models.RequestLog{}).Scopes(logFiltersScope(c))
}

// StreamLogKeysToCSV fetches unique keys from logs based on filters and streams them as a CSV.
func (s *LogService) StreamLogKeysToCSV(c *gin.Context, writer io.Writer) error {
	// Create a CSV writer
	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write CSV header
	header := []string{"key_value", "group_name", "status_code"}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	var results []ExportableLogKey

	baseQuery := s.DB.Model(&models.RequestLog{}).Scopes(logFiltersScope(c))

	// 使用窗口函数获取每个key_value的最新记录
	err := s.DB.Raw(`
		SELECT
			key_value,
			group_name,
			status_code
		FROM (
			SELECT
				key_value,
				group_name,
				status_code,
				ROW_NUMBER() OVER (PARTITION BY key_value ORDER BY timestamp DESC) as rn
			FROM (?) as filtered_logs
		) ranked
		WHERE rn = 1
		ORDER BY key_value
	`, baseQuery).Scan(&results).Error

	if err != nil {
		return fmt.Errorf("failed to fetch log keys: %w", err)
	}

	// 写入CSV数据
	for _, record := range results {
		csvRecord := []string{
			record.KeyValue,
			record.GroupName,
			strconv.Itoa(record.StatusCode),
		}
		if err := csvWriter.Write(csvRecord); err != nil {
			return fmt.Errorf("failed to write CSV record: %w", err)
		}
	}

	return nil
}

// DeleteLogsByIds deletes logs by their IDs.
func (s *LogService) DeleteLogsByIds(logIds []string) (int64, error) {
	if len(logIds) == 0 {
		return 0, nil
	}

	result := s.DB.Where("id IN ?", logIds).Delete(&models.RequestLog{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// ClearAllLogs clears all logs from the database.
func (s *LogService) ClearAllLogs() (int64, error) {
	result := s.DB.Where("1 = 1").Delete(&models.RequestLog{})
	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// CleanupDetailedContent 清理所有记录的详细内容但保留记录
func (s *LogService) CleanupDetailedContent() (int64, error) {
	result := s.DB.Model(&models.RequestLog{}).
		Where("(request_body != '' OR response_body != '' OR stream_content IS NOT NULL)").
		Updates(map[string]interface{}{
			"request_body":   "",
			"response_body":  "",
			"stream_content": nil,
		})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// CleanupLargeRecords 清理超过指定大小的记录详细内容
func (s *LogService) CleanupLargeRecords(maxSizeKB int) (int64, error) {
	maxSizeBytes := maxSizeKB * 1024

	result := s.DB.Model(&models.RequestLog{}).
		Where("(LENGTH(request_body) + LENGTH(response_body) + LENGTH(COALESCE(stream_content, ''))) > ?", maxSizeBytes).
		Updates(map[string]interface{}{
			"request_body":   "",
			"response_body":  "",
			"stream_content": nil,
		})

	if result.Error != nil {
		return 0, result.Error
	}

	return result.RowsAffected, nil
}

// CleanupByTimeRange 清理指定时间范围的记录
func (s *LogService) CleanupByTimeRange(startTime, endTime time.Time, onlyDetails bool) (int64, error) {
	query := s.DB.Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)

	if onlyDetails {
		// 只清理详细内容
		result := query.Model(&models.RequestLog{}).
			Where("(request_body != '' OR response_body != '' OR stream_content IS NOT NULL)").
			Updates(map[string]interface{}{
				"request_body":   "",
				"response_body":  "",
				"stream_content": nil,
			})
		return result.RowsAffected, result.Error
	} else {
		// 删除整条记录
		result := query.Delete(&models.RequestLog{})
		return result.RowsAffected, result.Error
	}
}

// CleanupByGroup 清理指定分组的记录
func (s *LogService) CleanupByGroup(groupName string, onlyDetails bool) (int64, error) {
	query := s.DB.Where("group_name = ?", groupName)

	if onlyDetails {
		// 只清理详细内容
		result := query.Model(&models.RequestLog{}).
			Where("(request_body != '' OR response_body != '' OR stream_content IS NOT NULL)").
			Updates(map[string]interface{}{
				"request_body":   "",
				"response_body":  "",
				"stream_content": nil,
			})
		return result.RowsAffected, result.Error
	} else {
		// 删除整条记录
		result := query.Delete(&models.RequestLog{})
		return result.RowsAffected, result.Error
	}
}

// GetDatabaseSize 获取数据库大小信息
func (s *LogService) GetDatabaseSize() (map[string]interface{}, error) {
	var logCount int64
	if err := s.DB.Model(&models.RequestLog{}).Count(&logCount).Error; err != nil {
		return nil, err
	}

	var totalContentSize int64
	if err := s.DB.Model(&models.RequestLog{}).
		Select("SUM(LENGTH(request_body) + LENGTH(response_body) + LENGTH(COALESCE(stream_content, '')))").
		Row().Scan(&totalContentSize); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"log_count":          logCount,
		"total_content_size": totalContentSize,
		"avg_size_per_log":   func() int64 {
			if logCount > 0 { return totalContentSize / logCount }
			return 0
		}(),
	}, nil
}
