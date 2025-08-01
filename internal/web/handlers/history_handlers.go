package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/Deali-Axy/ebook-generator/internal/web/services"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 全局历史服务实例
var historyService *services.HistoryService

// InitHistoryService 初始化历史服务
func InitHistoryService(hs *services.HistoryService) {
	historyService = hs
}

// GetHistories 获取转换历史列表
// @Summary 获取转换历史列表
// @Description 获取当前用户的转换历史记录列表
// @Tags 转换历史
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param status query string false "状态过滤"
// @Param format query string false "格式过滤"
// @Param start_date query string false "开始日期"
// @Param end_date query string false "结束日期"
// @Param keyword query string false "关键词搜索"
// @Param sort_by query string false "排序字段" default(created_at)
// @Param sort_order query string false "排序方向" default(desc)
// @Success 200 {object} models.APIResponse{data=models.HistoryListResponse}
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /history [get]
func GetHistories(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 绑定查询参数
	var req models.HistoryListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 获取历史记录
	response, err := historyService.GetUserHistories(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "获取历史记录失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取成功",
		Data:    response,
	})
}

// GetHistoryStats 获取转换统计信息
// @Summary 获取转换统计信息
// @Description 获取当前用户的转换统计信息
// @Tags 转换历史
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse{data=models.HistoryStatsResponse}
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /history/stats [get]
func GetHistoryStats(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取统计信息
	stats, err := historyService.GetUserStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "获取统计信息失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取成功",
		Data:    stats,
	})
}

// DeleteHistory 删除转换历史
// @Summary 删除转换历史
// @Description 删除指定的转换历史记录
// @Tags 转换历史
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "历史记录ID"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /history/{id} [delete]
func DeleteHistory(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取历史记录ID
	historyIDStr := c.Param("id")
	historyID, err := strconv.ParseUint(historyIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "无效的历史记录ID",
			Error:   err.Error(),
		})
		return
	}

	// 删除历史记录
	if err := historyService.DeleteHistory(userID, uint(historyID)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, models.APIResponse{
				Code:    404,
				Message: "历史记录不存在",
				Error:   err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, models.APIResponse{
				Code:    500,
				Message: "删除历史记录失败",
				Error:   err.Error(),
			})
		}
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "删除成功",
	})
}

// CreatePreset 创建转换预设
// @Summary 创建转换预设
// @Description 创建新的转换预设配置
// @Tags 转换预设
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.PresetCreateRequest true "预设信息"
// @Success 200 {object} models.APIResponse{data=models.ConversionPreset}
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /presets [post]
func CreatePreset(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 绑定请求参数
	var req models.PresetCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 创建预设
	preset, err := historyService.CreatePreset(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "创建预设失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "创建成功",
		Data:    preset,
	})
}

// GetPresets 获取转换预设列表
// @Summary 获取转换预设列表
// @Description 获取当前用户的转换预设列表
// @Tags 转换预设
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Param format query string false "格式过滤"
// @Param keyword query string false "关键词搜索"
// @Param is_public query bool false "是否公开"
// @Param sort_by query string false "排序字段" default(created_at)
// @Param sort_order query string false "排序方向" default(desc)
// @Success 200 {object} models.APIResponse{data=[]models.ConversionPreset}
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /presets [get]
func GetPresets(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 绑定查询参数
	var req models.PresetListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 获取预设列表
	presets, err := historyService.GetUserPresets(userID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "获取预设列表失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取成功",
		Data:    presets,
	})
}

// GetPreset 获取单个转换预设
// @Summary 获取单个转换预设
// @Description 根据ID获取指定的转换预设
// @Tags 转换预设
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "预设ID"
// @Success 200 {object} models.APIResponse{data=models.ConversionPreset}
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 404 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /presets/{id} [get]
func GetPreset(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取预设ID
	presetIDStr := c.Param("id")
	presetID, err := strconv.ParseUint(presetIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "无效的预设ID",
			Error:   err.Error(),
		})
		return
	}

	// 获取预设
	preset, err := historyService.GetPreset(userID, uint(presetID))
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIResponse{
			Code:    404,
			Message: "预设不存在",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取成功",
		Data:    preset,
	})
}

// UpdatePreset 更新转换预设
// @Summary 更新转换预设
// @Description 更新指定的转换预设配置
// @Tags 转换预设
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "预设ID"
// @Param request body models.PresetUpdateRequest true "预设信息"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /presets/{id} [put]
func UpdatePreset(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取预设ID
	presetIDStr := c.Param("id")
	presetID, err := strconv.ParseUint(presetIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "无效的预设ID",
			Error:   err.Error(),
		})
		return
	}

	// 绑定请求参数
	var req models.PresetUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 更新预设
	if err := historyService.UpdatePreset(userID, uint(presetID), &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "更新预设失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "更新成功",
	})
}

// DeletePreset 删除转换预设
// @Summary 删除转换预设
// @Description 删除指定的转换预设
// @Tags 转换预设
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path int true "预设ID"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /presets/{id} [delete]
func DeletePreset(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取预设ID
	presetIDStr := c.Param("id")
	presetID, err := strconv.ParseUint(presetIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "无效的预设ID",
			Error:   err.Error(),
		})
		return
	}

	// 删除预设
	if err := historyService.DeletePreset(userID, uint(presetID)); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "删除预设失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "删除成功",
	})
}

// BatchConvert 批量转换
// @Summary 批量转换
// @Description 批量转换多个文件
// @Tags 批量转换
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.BatchConversionRequest true "批量转换请求"
// @Success 200 {object} models.APIResponse{data=models.BatchConversionResponse}
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /convert/batch [post]
func BatchConvert(c *gin.Context) {
	// 获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 绑定请求参数
	var req models.BatchConversionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// TODO: 实现批量转换逻辑
	// 这里需要根据实际的转换服务来实现
	// 暂时返回一个示例响应
	response := &models.BatchConversionResponse{
		BatchID:    "batch_" + strconv.FormatInt(time.Now().Unix(), 10),
		TotalFiles: len(req.Files),
		Status:     "processing",
		CreatedAt:  time.Now(),
		Tasks:      make([]models.BatchTaskInfo, len(req.Files)),
	}

	// 为每个文件创建任务信息
	for i, file := range req.Files {
		taskID := fmt.Sprintf("task_%d_%d", userID, time.Now().UnixNano())
		response.Tasks[i] = models.BatchTaskInfo{
			TaskID:   taskID,
			FileName: file.FileName,
			Status:   "pending",
		}
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "批量转换任务已创建",
		Data:    response,
	})
}