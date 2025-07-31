@echo off
chcp 65001 >nul
echo === ebook-generator API接口测试 ===
echo.

echo 检查Go环境...
go version >nul 2>&1
if errorlevel 1 (
    echo 错误: 未找到Go命令，请确保Go已正确安装并添加到PATH
    pause
    exit /b 1
)

echo Go环境检查通过
echo.

echo 设置测试环境...
set GIN_MODE=test
set GO_ENV=test
echo.

echo 运行简化API测试...
go test -v ./tests/simple_api_test.go -timeout 30s
if errorlevel 1 (
    echo.
    echo 测试失败，请检查上面的错误信息
    pause
    exit /b 1
)

echo.
echo 所有测试通过！
echo.

echo 生成测试覆盖率报告...
go test -v ./tests/simple_api_test.go -coverprofile=coverage.out -timeout 30s
if exist coverage.out (
    echo.
    echo 测试覆盖率:
    go tool cover -func=coverage.out
    echo.
    echo 生成HTML覆盖率报告...
    go tool cover -html=coverage.out -o coverage.html
    echo HTML覆盖率报告已生成: coverage.html
)

echo.
echo 测试脚本执行完成！
pause