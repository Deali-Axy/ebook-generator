package database

import (
	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"gorm.io/gorm"
)

// AutoMigrate 自动迁移数据库表结构
func AutoMigrate(db *gorm.DB) error {
	// 自动迁移所有数据表
	err := db.AutoMigrate(
		&models.User{},
		&models.ConversionHistory{},
		&models.ConversionPreset{},
		&models.DownloadRecord{},
	)
	if err != nil {
		return err
	}

	// 创建默认管理员用户（如果不存在）
	if err := createDefaultAdmin(db); err != nil {
		return err
	}

	return nil
}

// createDefaultAdmin 创建默认管理员用户
func createDefaultAdmin(db *gorm.DB) error {
	// 检查是否已存在管理员用户
	var count int64
	db.Model(&models.User{}).Where("role = ?", models.RoleAdmin).Count(&count)
	if count > 0 {
		return nil // 已存在管理员用户
	}

	// 创建默认管理员
	admin := &models.User{
		Username: "admin",
		Email:    "admin@example.com",
		Password: "$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi", // password: password
		Role:     models.RoleAdmin,
		IsActive: true,
	}

	result := db.Create(admin)
	return result.Error
}