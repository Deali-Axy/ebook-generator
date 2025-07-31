package main

import (
	"log"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/Deali-Axy/ebook-generator/internal/mcp"
)

var version string = "1.0.0"

func main() {
	dir := os.Getenv("KAF_DIR")
	if dir != "" {
		err := os.Chdir(dir)
		if err != nil {
			panic(err)
		}
	}
	mcp.InitLogger()
	srv := server.NewMCPServer(
		"KAF Converter",
		version,
		server.WithLogging(),
	)

	// 注册工具
	converter := &mcp.ConverterService{}
	converter.RegisterTools(srv, version)

	// 启动服务
	if err := server.ServeStdio(srv); err != nil {
		log.Fatal("Server error: ", err)
	}
}
