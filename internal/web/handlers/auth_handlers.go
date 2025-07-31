package handlers

import (
	"fmt"
	"net/http"

	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/Deali-Axy/ebook-generator/internal/web/services"
	"github.com/gin-gonic/gin"
)

// 全局认证服务实例
var authService *services.AuthService

// InitAuthService 初始化认证服务
func InitAuthService(as *services.AuthService) {
	authService = as
}

// Register 用户注册
// @Summary 用户注册
// @Description 注册新用户账户
// @Tags 用户认证
// @Accept json
// @Produce json
// @Param request body models.RegisterRequest true "注册信息"
// @Success 200 {object} models.APIResponse{data=models.UserProfile}
// @Failure 400 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /auth/register [post]
func Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 注册用户
	user, err := authService.Register(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "注册失败",
			Error:   err.Error(),
		})
		return
	}

	// 返回用户资料（不包含密码）
	profile := &models.UserProfile{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		LastLogin: user.LastLogin,
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "注册成功",
		Data:    profile,
	})
}

// Login 用户登录
// @Summary 用户登录
// @Description 用户登录获取访问令牌
// @Tags 用户认证
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "登录信息"
// @Success 200 {object} models.APIResponse{data=models.LoginResponse}
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Router /auth/login [post]
func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 用户登录
	response, err := authService.Login(&req)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "登录失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "登录成功",
		Data:    response,
	})
}

// GetProfile 获取用户资料
// @Summary 获取用户资料
// @Description 获取当前登录用户的资料信息
// @Tags 用户认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse{data=models.UserProfile}
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /auth/profile [get]
func GetProfile(c *gin.Context) {
	// 从JWT中间件获取用户ID
	userID, err := getUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "用户未认证",
			Error:   err.Error(),
		})
		return
	}

	// 获取用户资料
	profile, err := authService.GetUserProfile(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "获取用户资料失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "获取成功",
		Data:    profile,
	})
}

// UpdateProfile 更新用户资料
// @Summary 更新用户资料
// @Description 更新当前用户的资料信息
// @Tags 用户认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.UserProfile true "用户资料"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /auth/profile [put]
func UpdateProfile(c *gin.Context) {
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
	var req models.UserProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 更新用户资料
	if err := authService.UpdateProfile(userID, &req); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "更新资料失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "资料更新成功",
	})
}

// Logout 用户登出
// @Summary 用户登出
// @Description 用户登出，使token失效
// @Tags 用户认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Router /auth/logout [post]
func Logout(c *gin.Context) {
	// 获取token
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" || len(authHeader) <= 7 {
		c.JSON(http.StatusUnauthorized, models.APIResponse{
			Code:    401,
			Message: "缺少认证token",
		})
		return
	}

	tokenString := authHeader[7:] // 移除"Bearer "

	// 将token加入黑名单
	if err := authService.RevokeToken(tokenString); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "登出失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "登出成功",
	})
}

// RefreshToken 刷新访问令牌
// @Summary 刷新访问令牌
// @Description 使用当前token刷新获取新的访问令牌
// @Tags 用户认证
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.APIResponse{data=models.LoginResponse}
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /auth/refresh [post]
func RefreshToken(c *gin.Context) {
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

	// 刷新token
	response, err := authService.RefreshToken(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIResponse{
			Code:    500,
			Message: "刷新token失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "刷新成功",
		Data:    response,
	})
}

// ChangePassword 修改密码
// @Summary 修改密码
// @Description 修改当前用户的密码
// @Tags 用户认证
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body models.ChangePasswordRequest true "密码信息"
// @Success 200 {object} models.APIResponse
// @Failure 400 {object} models.APIResponse
// @Failure 401 {object} models.APIResponse
// @Failure 500 {object} models.APIResponse
// @Router /auth/password [put]
func ChangePassword(c *gin.Context) {
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
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "请求参数错误",
			Error:   err.Error(),
		})
		return
	}

	// 修改密码
	if err := authService.ChangePassword(userID, &req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIResponse{
			Code:    400,
			Message: "修改密码失败",
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, models.APIResponse{
		Code:    200,
		Message: "密码修改成功",
	})
}

// AuthMiddleware JWT认证中间件
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从请求头获取token
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Code:    401,
				Message: "缺少认证token",
				Error:   "Authorization header required",
			})
			c.Abort()
			return
		}

		// 检查Bearer前缀
		tokenString := ""
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		} else {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Code:    401,
				Message: "无效的认证格式",
				Error:   "Invalid authorization format",
			})
			c.Abort()
			return
		}

		// 验证token
		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, models.APIResponse{
				Code:    401,
				Message: "无效的token",
				Error:   err.Error(),
			})
			c.Abort()
			return
		}

		// 将用户信息存储到上下文
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// getUserIDFromContext 从上下文获取用户ID
func getUserIDFromContext(c *gin.Context) (uint, error) {
	userIDInterface, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("用户ID不存在")
	}

	userID, ok := userIDInterface.(uint)
	if !ok {
		return 0, fmt.Errorf("用户ID类型错误")
	}

	return userID, nil
}