package validation

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileValidationResult 文件验证结果
type FileValidationResult struct {
	IsValid     bool   `json:"is_valid"`
	FileType    string `json:"file_type"`
	MimeType    string `json:"mime_type"`
	Encoding    string `json:"encoding"`
	Size        int64  `json:"size"`
	Error       string `json:"error,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
}

// FileValidator 文件验证器
type FileValidator struct {
	maxFileSize   int64
	allowedTypes  map[string]bool
	requireUTF8   bool
}

// NewFileValidator 创建文件验证器
func NewFileValidator(maxFileSize int64, allowedTypes []string, requireUTF8 bool) *FileValidator {
	typeMap := make(map[string]bool)
	for _, t := range allowedTypes {
		typeMap[strings.ToLower(t)] = true
	}

	return &FileValidator{
		maxFileSize:  maxFileSize,
		allowedTypes: typeMap,
		requireUTF8:  requireUTF8,
	}
}

// ValidateFile 验证上传的文件
func (fv *FileValidator) ValidateFile(fileHeader *multipart.FileHeader) (*FileValidationResult, error) {
	result := &FileValidationResult{
		Size: fileHeader.Size,
	}

	// 检查文件大小
	if fileHeader.Size > fv.maxFileSize {
		result.Error = fmt.Sprintf("文件大小超过限制，最大允许 %d 字节", fv.maxFileSize)
		return result, nil
	}

	// 打开文件
	file, err := fileHeader.Open()
	if err != nil {
		result.Error = fmt.Sprintf("无法打开文件: %v", err)
		return result, nil
	}
	defer file.Close()

	// 读取文件头部用于MIME类型检测
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		result.Error = fmt.Sprintf("读取文件失败: %v", err)
		return result, nil
	}
	buffer = buffer[:n]

	// 检测MIME类型
	mimeType := http.DetectContentType(buffer)
	result.MimeType = mimeType

	// 重置文件指针
	file.Seek(0, 0)

	// 根据文件扩展名和内容进行验证
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	switch ext {
	case ".txt":
		return fv.validateTextFile(file, result)
	case ".md":
		return fv.validateMarkdownFile(file, result)
	case ".html", ".htm":
		return fv.validateHTMLFile(file, result)
	case ".epub":
		return fv.validateEPUBFile(file, result)
	case ".mobi":
		return fv.validateMOBIFile(file, result)
	case ".azw3":
		return fv.validateAZW3File(file, result)
	default:
		if !fv.allowedTypes[ext] {
			result.Error = fmt.Sprintf("不支持的文件类型: %s", ext)
			return result, nil
		}
	}

	result.IsValid = true
	return result, nil
}

// validateTextFile 验证文本文件
func (fv *FileValidator) validateTextFile(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "text"

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件内容失败: %v", err)
		return result, nil
	}

	// 检查是否为有效的UTF-8编码
	if fv.requireUTF8 && !utf8.Valid(content) {
		result.Error = "文件不是有效的UTF-8编码"
		return result, nil
	}

	// 检查是否包含二进制内容
	if containsBinaryContent(content) {
		result.Error = "文件包含二进制内容，不是有效的文本文件"
		return result, nil
	}

	// 检查文件是否为空
	if len(strings.TrimSpace(string(content))) == 0 {
		result.Warnings = append(result.Warnings, "文件内容为空")
	}

	result.Encoding = "UTF-8"
	result.IsValid = true
	return result, nil
}

// validateMarkdownFile 验证Markdown文件
func (fv *FileValidator) validateMarkdownFile(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "markdown"

	// Markdown文件本质上也是文本文件
	return fv.validateTextFile(file, result)
}

// validateHTMLFile 验证HTML文件
func (fv *FileValidator) validateHTMLFile(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "html"

	// 读取文件内容
	content, err := io.ReadAll(file)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件内容失败: %v", err)
		return result, nil
	}

	// 检查UTF-8编码
	if fv.requireUTF8 && !utf8.Valid(content) {
		result.Error = "文件不是有效的UTF-8编码"
		return result, nil
	}

	// 简单的HTML标签检查
	htmlContent := string(content)
	if !strings.Contains(strings.ToLower(htmlContent), "<html") && 
	   !strings.Contains(strings.ToLower(htmlContent), "<!doctype") {
		result.Warnings = append(result.Warnings, "文件可能不是标准的HTML格式")
	}

	result.Encoding = "UTF-8"
	result.IsValid = true
	return result, nil
}

// validateEPUBFile 验证EPUB文件
func (fv *FileValidator) validateEPUBFile(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "epub"

	// 读取文件头部
	header := make([]byte, 4)
	_, err := file.Read(header)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件头失败: %v", err)
		return result, nil
	}

	// EPUB文件实际上是ZIP文件
	if !bytes.Equal(header, []byte{0x50, 0x4B, 0x03, 0x04}) && // ZIP文件头
	   !bytes.Equal(header, []byte{0x50, 0x4B, 0x05, 0x06}) {   // 空ZIP文件头
		result.Error = "文件不是有效的EPUB格式（ZIP压缩包）"
		return result, nil
	}

	result.IsValid = true
	return result, nil
}

// validateMOBIFile 验证MOBI文件
func (fv *FileValidator) validateMOBIFile(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "mobi"

	// 读取MOBI文件头
	header := make([]byte, 68)
	_, err := file.Read(header)
	if err != nil {
		result.Error = fmt.Sprintf("读取文件头失败: %v", err)
		return result, nil
	}

	// 检查MOBI标识
	if !bytes.Contains(header, []byte("BOOKMOBI")) {
		result.Error = "文件不是有效的MOBI格式"
		return result, nil
	}

	result.IsValid = true
	return result, nil
}

// validateAZW3File 验证AZW3文件
func (fv *FileValidator) validateAZW3File(file multipart.File, result *FileValidationResult) (*FileValidationResult, error) {
	result.FileType = "azw3"

	// AZW3文件格式类似MOBI
	return fv.validateMOBIFile(file, result)
}

// containsBinaryContent 检查内容是否包含二进制数据
func containsBinaryContent(content []byte) bool {
	// 检查是否包含NULL字节或其他控制字符
	for _, b := range content {
		if b == 0 || (b < 32 && b != 9 && b != 10 && b != 13) {
			return true
		}
	}
	return false
}

// ValidateFileContent 验证文件内容的便捷函数
func ValidateFileContent(fileHeader *multipart.FileHeader, maxSize int64, allowedTypes []string) (*FileValidationResult, error) {
	validator := NewFileValidator(maxSize, allowedTypes, true)
	return validator.ValidateFile(fileHeader)
}