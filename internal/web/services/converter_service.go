package services

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Deali-Axy/ebook-generator/internal/converter"
	"github.com/Deali-Axy/ebook-generator/internal/model"
	"github.com/Deali-Axy/ebook-generator/internal/web/types"
)

// ConverterService 转换器服务
type ConverterService struct {
	outputDir string
}

// NewConverterService 创建转换器服务
func NewConverterService(outputDir string) *ConverterService {
	return &ConverterService{
		outputDir: outputDir,
	}
}

// ConvertBook 转换电子书
func (s *ConverterService) ConvertBook(book *model.Book, format string) ([]types.ConvertedFileInfo, error) {
	var results []types.ConvertedFileInfo
	var formats []string

	// 确定要转换的格式
	if format == "all" {
		formats = []string{"epub", "mobi", "azw3"}
	} else {
		formats = []string{format}
	}

	// 设置输出目录
	originalOut := book.Out
	baseOut := filepath.Join(s.outputDir, originalOut)

	// 为每种格式进行转换
	for _, formatType := range formats {
		// 设置当前格式
		book.Format = formatType
		book.Out = baseOut

		// 获取对应的转换器
		conv, err := s.getConverter(formatType)
		if err != nil {
			return nil, fmt.Errorf("获取%s转换器失败: %w", formatType, err)
		}

		// 执行转换
		if err := conv.Build(*book); err != nil {
			return nil, fmt.Errorf("%s转换失败: %w", formatType, err)
		}

		// 构建输出文件路径
		outputFile := s.getOutputFilePath(baseOut, formatType)
		
		// 获取文件大小
		size, err := s.getFileSize(outputFile)
		if err != nil {
			return nil, fmt.Errorf("获取%s文件大小失败: %w", formatType, err)
		}

		// 添加到结果列表
		results = append(results, types.ConvertedFileInfo{
			Format:   formatType,
			Filename: filepath.Base(outputFile),
			Path:     outputFile,
			Size:     size,
		})
	}

	// 恢复原始输出设置
	book.Out = originalOut

	return results, nil
}

// getConverter 获取指定格式的转换器
func (s *ConverterService) getConverter(format string) (converter.Converter, error) {
	switch strings.ToLower(format) {
	case "epub":
		return &converter.EpubConverter{}, nil
	case "mobi":
		return &converter.MobiConverter{}, nil
	case "azw3":
		return &converter.Azw3Converter{}, nil
	default:
		return nil, fmt.Errorf("不支持的格式: %s", format)
	}
}

// getOutputFilePath 获取输出文件路径
func (s *ConverterService) getOutputFilePath(basePath, format string) string {
	switch strings.ToLower(format) {
	case "epub":
		return basePath + ".epub"
	case "mobi":
		return basePath + ".mobi"
	case "azw3":
		return basePath + ".azw3"
	default:
		return basePath + "." + format
	}
}

// getFileSize 获取文件大小
func (s *ConverterService) getFileSize(filePath string) (int64, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return 0, fmt.Errorf("获取文件信息失败: %w", err)
	}
	return stat.Size(), nil
}

// GetSupportedFormats 获取支持的格式列表
func (s *ConverterService) GetSupportedFormats() []string {
	return []string{"epub", "mobi", "azw3", "all"}
}

// ValidateFormat 验证格式是否支持
func (s *ConverterService) ValidateFormat(format string) bool {
	supportedFormats := s.GetSupportedFormats()
	for _, supported := range supportedFormats {
		if strings.ToLower(format) == strings.ToLower(supported) {
			return true
		}
	}
	return false
}