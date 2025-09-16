package services

import (
	"context"
	"gpt-load/internal/config"
	"gpt-load/internal/models"
	"gpt-load/internal/types"
	"os"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// LogCleanupService 负责清理过期的请求日志
type LogCleanupService struct {
	db              *gorm.DB
	settingsManager *config.SystemSettingsManager
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewLogCleanupService 创建新的日志清理服务
func NewLogCleanupService(db *gorm.DB, settingsManager *config.SystemSettingsManager) *LogCleanupService {
	return &LogCleanupService{
		db:              db,
		settingsManager: settingsManager,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动日志清理服务
func (s *LogCleanupService) Start() {
	s.wg.Add(1)
	go s.run()
	logrus.Debug("Log cleanup service started")
}

// Stop 停止日志清理服务
func (s *LogCleanupService) Stop(ctx context.Context) {
	close(s.stopCh)

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logrus.Info("LogCleanupService stopped gracefully.")
	case <-ctx.Done():
		logrus.Warn("LogCleanupService stop timed out.")
	}
}

// run 运行日志清理的主循环
func (s *LogCleanupService) run() {
	defer s.wg.Done()

	// 使用配置的清理频率
	settings := s.settingsManager.GetSettings()
	intervalMinutes := settings.CleanupFrequencyMinutes
	if intervalMinutes < 5 {
		intervalMinutes = 30 // 默认30分钟
	}

	ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
	defer ticker.Stop()

	// 启动时先执行一次清理
	s.performFullCleanup()

	for {
		select {
		case <-ticker.C:
			s.performFullCleanup()
		case <-s.stopCh:
			return
		}
	}
}

// performFullCleanup 执行完整的清理流程
func (s *LogCleanupService) performFullCleanup() {
	settings := s.settingsManager.GetSettings()

	// 1. 检查数据库大小，如果超过阈值则触发额外清理
	if settings.CleanupTriggerDbSizeMB > 0 {
		if s.isDatabaseOversized(settings.CleanupTriggerDbSizeMB) {
			logrus.Info("Database size exceeded threshold, performing aggressive cleanup")
			s.cleanupLargeRecords(settings)
		}
	}

	// 2. 清理详细内容（分层清理第1层）
	s.cleanupDetailedContent(settings)

	// 3. 清理过期记录（分层清理第2层）
	s.cleanupExpiredLogs(settings)

	// 4. 执行数据库VACUUM来回收空间
	s.vacuumDatabase()
}

// isDatabaseOversized 检查数据库是否超过大小阈值
func (s *LogCleanupService) isDatabaseOversized(thresholdMB int) bool {
	var dbPath string
	s.db.Raw("PRAGMA database_list").Row().Scan(nil, nil, &dbPath)

	if dbPath == "" {
		return false
	}

	if stat, err := os.Stat(dbPath); err == nil {
		sizeMB := stat.Size() / 1024 / 1024
		return sizeMB > int64(thresholdMB)
	}
	return false
}

// cleanupDetailedContent 清理详细内容但保留记录摘要
func (s *LogCleanupService) cleanupDetailedContent(settings types.SystemSettings) {
	if settings.DetailedLogRetentionHours <= 0 {
		logrus.Debug("Detailed content cleanup is disabled")
		return
	}

	// 计算详细内容过期时间点
	cutoffTime := time.Now().Add(-time.Duration(settings.DetailedLogRetentionHours) * time.Hour).UTC()

	// 清理详细内容，但保留记录
	result := s.db.Model(&models.RequestLog{}).
		Where("timestamp < ?", cutoffTime).
		Where("(request_body != '' OR response_body != '' OR stream_content IS NOT NULL)").
		Updates(map[string]interface{}{
			"request_body":   "",
			"response_body":  "",
			"stream_content": nil,
		})

	if result.Error != nil {
		logrus.WithError(result.Error).Error("Failed to cleanup detailed log content")
		return
	}

	if result.RowsAffected > 0 {
		logrus.WithFields(logrus.Fields{
			"cleaned_count":   result.RowsAffected,
			"cutoff_time":     cutoffTime.Format(time.RFC3339),
			"retention_hours": settings.DetailedLogRetentionHours,
		}).Info("Successfully cleaned up detailed log content")
	}
}

// cleanupLargeRecords 清理大记录的详细内容
func (s *LogCleanupService) cleanupLargeRecords(settings types.SystemSettings) {
	maxSizeBytes := settings.MaxRequestBodySizeKB * 1024

	// 清理超大记录的详细内容
	result := s.db.Model(&models.RequestLog{}).
		Where("(LENGTH(request_body) + LENGTH(response_body) + LENGTH(COALESCE(stream_content, ''))) > ?", maxSizeBytes).
		Updates(map[string]interface{}{
			"request_body":   "",
			"response_body":  "",
			"stream_content": nil,
		})

	if result.Error != nil {
		logrus.WithError(result.Error).Error("Failed to cleanup large records")
		return
	}

	if result.RowsAffected > 0 {
		logrus.WithFields(logrus.Fields{
			"cleaned_count": result.RowsAffected,
			"max_size_kb":   settings.MaxRequestBodySizeKB,
		}).Info("Successfully cleaned up large record details")
	}
}

// cleanupExpiredLogs 清理过期的请求日志记录
func (s *LogCleanupService) cleanupExpiredLogs(settings types.SystemSettings) {
	retentionDays := settings.RequestLogRetentionDays

	if retentionDays <= 0 {
		logrus.Debug("Log retention is disabled (retention_days <= 0)")
		return
	}

	// 计算过期时间点
	cutoffTime := time.Now().AddDate(0, 0, -retentionDays).UTC()

	// 执行删除操作
	result := s.db.Where("timestamp < ?", cutoffTime).Delete(&models.RequestLog{})
	if result.Error != nil {
		logrus.WithError(result.Error).Error("Failed to cleanup expired request logs")
		return
	}

	if result.RowsAffected > 0 {
		logrus.WithFields(logrus.Fields{
			"deleted_count":  result.RowsAffected,
			"cutoff_time":    cutoffTime.Format(time.RFC3339),
			"retention_days": retentionDays,
		}).Info("Successfully cleaned up expired request logs")
	}
}

// vacuumDatabase 执行数据库VACUUM操作回收空间
func (s *LogCleanupService) vacuumDatabase() {
	result := s.db.Exec("VACUUM")
	if result.Error != nil {
		logrus.WithError(result.Error).Warn("Failed to vacuum database")
	} else {
		logrus.Debug("Database vacuum completed successfully")
	}
}
