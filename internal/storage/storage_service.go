package storage

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/Deali-Axy/ebook-generator/internal/web/types"
)

// StorageService 存储服务
type StorageService struct {
	uploadDir   string // 上传目录
	outputDir   string // 输出目录
	maxFileSize int64  // 最大文件大小
}

// NewStorageService 创建存储服务
func NewStorageService(uploadDir, outputDir string, maxFileSize int64) *StorageService {
	// 确保目录存在
	os.MkdirAll(uploadDir, 0755)
	os.MkdirAll(outputDir, 0755)

	return &StorageService{
		uploadDir:   uploadDir,
		outputDir:   outputDir,
		maxFileSize: maxFileSize,
	}
}

// SaveUploadedFile 保存上传的文件
func (s *StorageService) SaveUploadedFile(taskID string, fileHeader *multipart.FileHeader) (*models.UploadResponse, error) {
	// 验证文件大小
	if fileHeader.Size > s.maxFileSize {
		return nil, fmt.Errorf("文件大小超过限制: %d bytes (最大: %d bytes)", fileHeader.Size, s.maxFileSize)
	}

	// 验证文件类型
	if !s.isValidTextFile(fileHeader.Filename) {
		return nil, fmt.Errorf("不支持的文件类型，仅支持.txt文件")
	}

	// 打开上传的文件
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer src.Close()

	// 创建目标文件路径
	filename := s.sanitizeFilename(fileHeader.Filename)
	destPath := filepath.Join(s.uploadDir, taskID+"_"+filename)

	// 创建目标文件
	dst, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dst.Close()

	// 复制文件内容
	if _, err := io.Copy(dst, src); err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	return &models.UploadResponse{
		TaskID:   taskID,
		Filename: filename,
		Size:     fileHeader.Size,
		UploadAt: time.Now().Format(time.RFC3339),
	}, nil
}

// GetUploadedFilePath 获取上传文件的路径
func (s *StorageService) GetUploadedFilePath(taskID string) (string, error) {
	// 查找以taskID开头的文件
	pattern := filepath.Join(s.uploadDir, taskID+"_*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("查找文件失败: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("未找到任务文件: %s", taskID)
	}

	return matches[0], nil
}

// SaveConvertedFiles 保存转换后的文件
func (s *StorageService) SaveConvertedFiles(taskID string, convertedFiles []types.ConvertedFileInfo) ([]models.ConvertedFile, error) {
	var results []models.ConvertedFile

	for _, file := range convertedFiles {
		// 生成文件ID
		fileID := s.generateFileID(taskID, file.Format)

		// 目标路径
		destPath := filepath.Join(s.outputDir, fileID+"_"+file.Filename)

		// 如果源文件和目标文件不同，则复制文件
		if file.Path != destPath {
			if err := s.copyFile(file.Path, destPath); err != nil {
				return nil, fmt.Errorf("复制文件失败 %s: %w", file.Filename, err)
			}
		}

		// 获取文件大小
		size, err := s.getFileSize(destPath)
		if err != nil {
			return nil, fmt.Errorf("获取文件大小失败 %s: %w", file.Filename, err)
		}

		results = append(results, models.ConvertedFile{
			FileID:   fileID,
			Format:   file.Format,
			Filename: file.Filename,
			Size:     size,
			Path:     destPath,
		})
	}

	return results, nil
}

// GetConvertedFile 获取转换后的文件信息
func (s *StorageService) GetConvertedFile(fileID string) (*models.ConvertedFile, error) {
	// 查找以fileID开头的文件
	pattern := filepath.Join(s.outputDir, fileID+"_*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("查找文件失败: %w", err)
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("文件不存在: %s", fileID)
	}

	filePath := matches[0]
	filename := filepath.Base(filePath)

	// 从文件名中提取格式
	format := s.extractFormatFromFilename(filename)

	// 获取文件大小
	size, err := s.getFileSize(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件大小失败: %w", err)
	}

	return &models.ConvertedFile{
		FileID:   fileID,
		Format:   format,
		Filename: filename,
		Size:     size,
		Path:     filePath,
	}, nil
}

// CleanupTask 清理任务相关的所有文件
func (s *StorageService) CleanupTask(taskID string) ([]string, error) {
	var cleanedFiles []string

	// 清理上传文件
	uploadPattern := filepath.Join(s.uploadDir, taskID+"_*")
	uploadMatches, err := filepath.Glob(uploadPattern)
	if err == nil {
		for _, file := range uploadMatches {
			if err := os.Remove(file); err == nil {
				cleanedFiles = append(cleanedFiles, file)
			}
		}
	}

	// 清理输出文件
	outputPattern := filepath.Join(s.outputDir, taskID+"_*")
	outputMatches, err := filepath.Glob(outputPattern)
	if err == nil {
		for _, file := range outputMatches {
			if err := os.Remove(file); err == nil {
				cleanedFiles = append(cleanedFiles, file)
			}
		}
	}

	return cleanedFiles, nil
}

// isValidTextFile 验证是否为有效的文本文件
func (s *StorageService) isValidTextFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".txt"
}

// sanitizeFilename 清理文件名
func (s *StorageService) sanitizeFilename(filename string) string {
	// 移除路径分隔符和其他危险字符
	filename = filepath.Base(filename)
	filename = strings.ReplaceAll(filename, "..", "")
	filename = strings.ReplaceAll(filename, "/", "")
	filename = strings.ReplaceAll(filename, "\\", "")
	return filename
}

// generateFileID 生成文件ID
func (s *StorageService) generateFileID(taskID, format string) string {
	return fmt.Sprintf("%s_%s_%d", taskID, format, time.Now().Unix())
}

// copyFile 复制文件
func (s *StorageService) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

// getFileSize 获取文件大小
func (s *StorageService) getFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// extractFormatFromFilename 从文件名中提取格式
func (s *StorageService) extractFormatFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	if len(ext) > 1 {
		return ext[1:] // 移除点号
	}
	return "unknown"
}

// GetContentType 根据文件格式获取Content-Type
func (s *StorageService) GetContentType(format string) string {
	switch strings.ToLower(format) {
	case "epub":
		return "application/epub+zip"
	case "mobi":
		return "application/x-mobipocket-ebook"
	case "azw3":
		return "application/vnd.amazon.ebook"
	default:
		return "application/octet-stream"
	}
}
