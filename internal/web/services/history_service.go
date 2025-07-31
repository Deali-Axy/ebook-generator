package services

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"gorm.io/gorm"
)

// HistoryService 历史记录服务
type HistoryService struct {
	db *gorm.DB
}

// NewHistoryService 创建历史记录服务
func NewHistoryService(db *gorm.DB) *HistoryService {
	return &HistoryService{
		db: db,
	}
}

// CreateHistory 创建转换历史记录
func (hs *HistoryService) CreateHistory(userID uint, taskID string, originalFileName string, originalFileSize int64, originalFileHash string, outputFormat string, options map[string]interface{}) (*models.ConversionHistory, error) {
	history := &models.ConversionHistory{
		UserID:           userID,
		TaskID:           taskID,
		OriginalFileName: originalFileName,
		OriginalFileSize: originalFileSize,
		OriginalFileHash: originalFileHash,
		OutputFormat:     outputFormat,
		ConvertOptions:   models.ConvertOptionsJSON(options),
		Status:           "processing",
		StartTime:        time.Now(),
	}

	result := hs.db.Create(history)
	return history, result.Error
}

// UpdateHistoryStatus 更新历史记录状态
func (hs *HistoryService) UpdateHistoryStatus(taskID string, status string, errorMessage string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == "completed" {
		now := time.Now()
		updates["end_time"] = &now
	} else if status == "failed" {
		now := time.Now()
		updates["end_time"] = &now
		updates["error_message"] = errorMessage
	}

	result := hs.db.Model(&models.ConversionHistory{}).Where("task_id = ?", taskID).Updates(updates)
	return result.Error
}

// CompleteHistory 完成转换历史记录
func (hs *HistoryService) CompleteHistory(taskID string, outputFileName string, outputFileSize int64) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":           "completed",
		"end_time":         &now,
		"output_file_name": outputFileName,
		"output_file_size": outputFileSize,
	}

	// 计算持续时间
	var history models.ConversionHistory
	if err := hs.db.Where("task_id = ?", taskID).First(&history).Error; err == nil {
		updates["duration"] = now.Sub(history.StartTime).Milliseconds()
	}

	result := hs.db.Model(&models.ConversionHistory{}).Where("task_id = ?", taskID).Updates(updates)
	return result.Error
}

// GetHistoryByTaskID 根据任务ID获取历史记录
func (hs *HistoryService) GetHistoryByTaskID(taskID string) (*models.ConversionHistory, error) {
	var history models.ConversionHistory
	result := hs.db.Where("task_id = ? AND is_deleted = ?", taskID, false).First(&history)
	if result.Error != nil {
		return nil, result.Error
	}
	return &history, nil
}

// GetUserHistories 获取用户历史记录列表
func (hs *HistoryService) GetUserHistories(userID uint, req *models.HistoryListRequest) (*models.HistoryListResponse, error) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// 构建查询
	query := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND is_deleted = ?", userID, false)

	// 添加过滤条件
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Format != "" {
		query = query.Where("output_format = ?", req.Format)
	}
	if req.StartDate != "" {
		query = query.Where("created_at >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		query = query.Where("created_at <= ?", req.EndDate)
	}
	if req.Keyword != "" {
		query = query.Where("original_file_name LIKE ? OR output_file_name LIKE ?", 
			"%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}

	// 获取总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	// 添加排序和分页
	orderClause := fmt.Sprintf("%s %s", req.SortBy, req.SortOrder)
	offset := (req.Page - 1) * req.PageSize

	var histories []models.ConversionHistory
	if err := query.Order(orderClause).Offset(offset).Limit(req.PageSize).Find(&histories).Error; err != nil {
		return nil, err
	}

	// 计算总页数
	totalPages := int((total + int64(req.PageSize) - 1) / int64(req.PageSize))

	return &models.HistoryListResponse{
		Items:      histories,
		Total:      total,
		Page:       req.Page,
		PageSize:   req.PageSize,
		TotalPages: totalPages,
	}, nil
}

// GetUserStats 获取用户统计信息
func (hs *HistoryService) GetUserStats(userID uint) (*models.HistoryStatsResponse, error) {
	stats := &models.HistoryStatsResponse{
		FormatStats: make(map[string]int64),
	}

	// 总转换次数
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND is_deleted = ?", userID, false).Count(&stats.TotalConversions).Error; err != nil {
		return nil, err
	}

	// 成功转换次数
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND status = ? AND is_deleted = ?", userID, "completed", false).Count(&stats.SuccessfulConversions).Error; err != nil {
		return nil, err
	}

	// 失败转换次数
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND status = ? AND is_deleted = ?", userID, "failed", false).Count(&stats.FailedConversions).Error; err != nil {
		return nil, err
	}

	// 计算成功率
	if stats.TotalConversions > 0 {
		stats.SuccessRate = float64(stats.SuccessfulConversions) / float64(stats.TotalConversions) * 100
	}

	// 总文件大小
	var totalSize sql.NullInt64
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND is_deleted = ?", userID, false).Select("SUM(original_file_size)").Scan(&totalSize).Error; err != nil {
		return nil, err
	}
	if totalSize.Valid {
		stats.TotalFileSize = totalSize.Int64
	}

	// 平均转换时间
	var avgDuration sql.NullFloat64
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND status = ? AND is_deleted = ?", userID, "completed", false).Select("AVG(duration)").Scan(&avgDuration).Error; err != nil {
		return nil, err
	}
	if avgDuration.Valid {
		stats.AverageDuration = avgDuration.Float64
	}

	// 格式统计
	var formatStats []struct {
		OutputFormat string `json:"output_format"`
		Count        int64  `json:"count"`
	}
	if err := hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND is_deleted = ?", userID, false).Select("output_format, COUNT(*) as count").Group("output_format").Scan(&formatStats).Error; err != nil {
		return nil, err
	}
	for _, stat := range formatStats {
		stats.FormatStats[stat.OutputFormat] = stat.Count
	}

	// 月度统计（最近12个月）
	monthlyStats, err := hs.getMonthlyStats(userID, 12)
	if err != nil {
		return nil, err
	}
	stats.MonthlyStats = monthlyStats

	// 最近活动（最近10条）
	var recentActivity []models.ConversionHistory
	if err := hs.db.Where("user_id = ? AND is_deleted = ?", userID, false).Order("created_at DESC").Limit(10).Find(&recentActivity).Error; err != nil {
		return nil, err
	}
	stats.RecentActivity = recentActivity

	return stats, nil
}

// getMonthlyStats 获取月度统计
func (hs *HistoryService) getMonthlyStats(userID uint, months int) ([]models.MonthlyStats, error) {
	var stats []models.MonthlyStats

	// 这里需要根据具体数据库实现月度统计查询
	// 以下是一个简化的实现
	for i := months - 1; i >= 0; i-- {
		date := time.Now().AddDate(0, -i, 0)
		startOfMonth := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
		endOfMonth := startOfMonth.AddDate(0, 1, 0).Add(-time.Nanosecond)

		var total, success, failure int64

		// 总数
		hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND created_at >= ? AND created_at <= ? AND is_deleted = ?", 
			userID, startOfMonth, endOfMonth, false).Count(&total)

		// 成功数
		hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND status = ? AND created_at >= ? AND created_at <= ? AND is_deleted = ?", 
			userID, "completed", startOfMonth, endOfMonth, false).Count(&success)

		// 失败数
		hs.db.Model(&models.ConversionHistory{}).Where("user_id = ? AND status = ? AND created_at >= ? AND created_at <= ? AND is_deleted = ?", 
			userID, "failed", startOfMonth, endOfMonth, false).Count(&failure)

		stats = append(stats, models.MonthlyStats{
			Month:        date.Format("01"),
			Year:         date.Year(),
			Count:        total,
			SuccessCount: success,
			FailureCount: failure,
		})
	}

	return stats, nil
}

// DeleteHistory 删除历史记录（软删除）
func (hs *HistoryService) DeleteHistory(userID uint, historyID uint) error {
	result := hs.db.Model(&models.ConversionHistory{}).Where("id = ? AND user_id = ?", historyID, userID).Update("is_deleted", true)
	return result.Error
}

// RecordDownload 记录下载
func (hs *HistoryService) RecordDownload(userID uint, taskID string, fileName string, fileSize int64, clientIP string, userAgent string) error {
	// 更新历史记录的下载次数
	if err := hs.db.Model(&models.ConversionHistory{}).Where("task_id = ?", taskID).Updates(map[string]interface{}{
		"download_count":    gorm.Expr("download_count + 1"),
		"last_download_at": time.Now(),
	}).Error; err != nil {
		return err
	}

	// 创建下载记录
	var historyID uint
	if err := hs.db.Model(&models.ConversionHistory{}).Where("task_id = ?", taskID).Select("id").Scan(&historyID).Error; err != nil {
		return err
	}

	downloadRecord := &models.DownloadRecord{
		UserID:     userID,
		HistoryID:  historyID,
		TaskID:     taskID,
		FileName:   fileName,
		FileSize:   fileSize,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		DownloadAt: time.Now(),
	}

	return hs.db.Create(downloadRecord).Error
}

// CreatePreset 创建转换预设
func (hs *HistoryService) CreatePreset(userID uint, req *models.PresetCreateRequest) (*models.ConversionPreset, error) {
	// 如果设置为默认预设，先取消其他默认预设
	if req.IsDefault {
		hs.db.Model(&models.ConversionPreset{}).Where("user_id = ? AND is_default = ?", userID, true).Update("is_default", false)
	}

	preset := &models.ConversionPreset{
		UserID:       userID,
		Name:         req.Name,
		Description:  req.Description,
		OutputFormat: req.OutputFormat,
		Options:      models.ConvertOptionsJSON(req.Options),
		IsDefault:    req.IsDefault,
		IsPublic:     req.IsPublic,
	}

	result := hs.db.Create(preset)
	return preset, result.Error
}

// GetUserPresets 获取用户预设列表
func (hs *HistoryService) GetUserPresets(userID uint, req *models.PresetListRequest) ([]models.ConversionPreset, error) {
	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.SortBy == "" {
		req.SortBy = "created_at"
	}
	if req.SortOrder == "" {
		req.SortOrder = "desc"
	}

	// 构建查询
	query := hs.db.Model(&models.ConversionPreset{}).Where("user_id = ? OR is_public = ?", userID, true)

	// 添加过滤条件
	if req.Format != "" {
		query = query.Where("output_format = ?", req.Format)
	}
	if req.Keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}
	if req.IsPublic != nil {
		query = query.Where("is_public = ?", *req.IsPublic)
	}

	// 添加排序和分页
	orderClause := fmt.Sprintf("%s %s", req.SortBy, req.SortOrder)
	offset := (req.Page - 1) * req.PageSize

	var presets []models.ConversionPreset
	result := query.Order(orderClause).Offset(offset).Limit(req.PageSize).Find(&presets)
	return presets, result.Error
}

// UpdatePreset 更新预设
func (hs *HistoryService) UpdatePreset(userID uint, presetID uint, req *models.PresetUpdateRequest) error {
	// 如果设置为默认预设，先取消其他默认预设
	if req.IsDefault {
		hs.db.Model(&models.ConversionPreset{}).Where("user_id = ? AND id != ? AND is_default = ?", userID, presetID, true).Update("is_default", false)
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.OutputFormat != "" {
		updates["output_format"] = req.OutputFormat
	}
	if req.Options != nil {
		updates["options"] = models.ConvertOptionsJSON(req.Options)
	}
	updates["is_default"] = req.IsDefault
	updates["is_public"] = req.IsPublic

	result := hs.db.Model(&models.ConversionPreset{}).Where("id = ? AND user_id = ?", presetID, userID).Updates(updates)
	return result.Error
}

// DeletePreset 删除预设
func (hs *HistoryService) DeletePreset(userID uint, presetID uint) error {
	result := hs.db.Where("id = ? AND user_id = ?", presetID, userID).Delete(&models.ConversionPreset{})
	return result.Error
}

// UsePreset 使用预设（增加使用次数）
func (hs *HistoryService) UsePreset(presetID uint) error {
	result := hs.db.Model(&models.ConversionPreset{}).Where("id = ?", presetID).Update("usage_count", gorm.Expr("usage_count + 1"))
	return result.Error
}