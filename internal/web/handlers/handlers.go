package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/storage"
	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/Deali-Axy/ebook-generator/internal/web/services"
	"github.com/gin-gonic/gin"
)

// 全局服务实例
var (
	taskService      *services.TaskService
	storageService   *storage.StorageService
	converterService *services.ConverterService
)

// InitServices 初始化服务
func InitServices(ts *services.TaskService, ss *storage.StorageService, cs *services.ConverterService) {
	taskService = ts
	storageService = ss
	converterService = cs
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("task_%d", time.Now().UnixNano())
}

// UploadFile 上传文件
// @Summary 上传txt文件
// @Description 上传txt文件用于转换
// @Tags 文件管理
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "txt文件"
// @Success 200 {object} models.APIResponse{data=models.UploadResponse}
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /upload [post]
func UploadFile(c *gin.Context) {
	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "获取上传文件失败",
			Error:   err.Error(),
		})
		return
	}

	// 生成任务ID
	taskID := generateTaskID()

	// 保存文件
	uploadResp, err := storageService.SaveUploadedFile(taskID, file)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "保存文件失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "文件上传成功",
		Data:    uploadResp,
	})
}

// ConvertBook 开始转换任务
// @Summary 开始电子书转换
// @Description 开始将txt文件转换为电子书
// @Tags 转换管理
// @Accept json
// @Produce json
// @Param request body models.ConvertRequest true "转换参数"
// @Success 200 {object} models.APIResponse{data=models.ConvertResponse}
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /convert [post]
func ConvertBook(c *gin.Context) {
	var req models.ConvertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 验证格式
	if !converterService.ValidateFormat(req.Format) {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "不支持的格式",
			Error:   fmt.Sprintf("支持的格式: %s", strings.Join(converterService.GetSupportedFormats(), ", ")),
		})
		return
	}

	// 检查任务是否存在
	if _, exists := taskService.GetTask(req.TaskID); exists {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "任务已存在",
			Error:   "Task already exists",
		})
		return
	}

	// 检查上传文件是否存在
	if _, err := storageService.GetUploadedFilePath(req.TaskID); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "未找到上传文件",
			Error:   err.Error(),
		})
		return
	}

	// 创建任务
	task := taskService.CreateTask(req.TaskID, &req)

	// 开始转换
	if err := taskService.StartConversion(req.TaskID); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "启动转换任务失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "转换任务已开始",
		Data: models.ConvertResponse{
			TaskID:    task.ID,
			Status:    task.Status,
			Message:   task.Message,
			StartedAt: task.StartedAt.Format(time.RFC3339),
		},
	})
}

// GetTaskStatus 获取任务状态
// @Summary 查询转换状态
// @Description 查询转换任务的当前状态
// @Tags 转换管理
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} models.APIResponse{data=models.TaskStatusResponse}
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /status/{taskId} [get]
func GetTaskStatus(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "任务ID不能为空",
			Error:   "Task ID is required",
		})
		return
	}

	status, err := taskService.GetTaskStatus(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "任务不存在",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取状态成功",
		Data:    status,
	})
}

// DownloadFile 下载转换后的文件
// @Summary 下载电子书文件
// @Description 下载转换后的电子书文件
// @Tags 文件管理
// @Produce application/octet-stream
// @Param fileId path string true "文件ID"
// @Success 200 {file} binary
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /download/{fileId} [get]
func DownloadFile(c *gin.Context) {
	fileID := c.Param("fileId")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "文件ID不能为空",
			Error:   "File ID is required",
		})
		return
	}

	// 获取文件信息
	file, err := storageService.GetConvertedFile(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "文件不存在",
			Error:   err.Error(),
		})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", file.Filename))
	c.Header("Content-Type", storageService.GetContentType(file.Format))
	c.Header("Content-Length", strconv.FormatInt(file.Size, 10))

	// 发送文件
	c.File(file.Path)
}

// CleanupTask 清理任务
// @Summary 清理任务文件
// @Description 清理任务相关的所有文件
// @Tags 任务管理
// @Produce json
// @Param taskId path string true "任务ID"
// @Success 200 {object} models.APIResponse{data=models.CleanupResponse}
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /cleanup/{taskId} [delete]
func CleanupTask(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "任务ID不能为空",
			Error:   "Task ID is required",
		})
		return
	}

	response, err := taskService.CleanupTask(taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "清理失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "清理成功",
		Data:    response,
	})
}

// GetTaskEvents SSE接口，实时获取任务事件
// @Summary 获取任务事件流
// @Description 通过SSE获取任务的实时事件流
// @Tags 转换管理
// @Produce text/event-stream
// @Param taskId path string true "任务ID"
// @Success 200 {string} string "SSE事件流"
// @Failure 404 {object} models.APIResponse
// @Router /events/{taskId} [get]
func GetTaskEvents(c *gin.Context) {
	taskID := c.Param("taskId")
	if taskID == "" {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "任务ID不能为空",
			Error:   "Task ID is required",
		})
		return
	}

	// 检查任务是否存在
	if _, exists := taskService.GetTask(taskID); !exists {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "任务不存在",
			Error:   "Task not found",
		})
		return
	}

	// 获取事件通道
	eventChan, exists := taskService.GetEventChannel(taskID)
	if !exists {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "事件通道不存在",
			Error:   "Event channel not found",
		})
		return
	}

	// 设置SSE响应头
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "Streaming not supported",
		})
		return
	}

	// 循环处理事件
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// 通道已关闭
				return
			}
			// 手动发送SSE事件
			jsonData, err := json.Marshal(event)
			if err != nil {
				// 在实际应用中，这里应该记录日志
				continue
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", string(jsonData))
			flusher.Flush()
		case <-time.After(30 * time.Second):
			// 超时，发送心跳
			fmt.Fprintf(c.Writer, "event: ping\ndata: heartbeat\n\n")
			flusher.Flush()
		case <-c.Request.Context().Done():
			// 客户端断开连接
			return
		}
	}
}
