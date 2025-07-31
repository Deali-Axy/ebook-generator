package types

// ConvertedFileInfo 转换后的文件信息
type ConvertedFileInfo struct {
	Format   string // 格式
	Filename string // 文件名
	Path     string // 完整路径
	Size     int64  // 文件大小
}