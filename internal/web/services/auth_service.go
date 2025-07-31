package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/Deali-Axy/ebook-generator/internal/web/models"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AuthService 认证服务
type AuthService struct {
	db        *gorm.DB
	jwtSecret []byte
	tokenTTL  time.Duration
}

// JWTClaims JWT声明
type JWTClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// NewAuthService 创建认证服务
func NewAuthService(db *gorm.DB, jwtSecret string, tokenTTL time.Duration) *AuthService {
	return &AuthService{
		db:        db,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

// Register 用户注册
func (s *AuthService) Register(req *models.RegisterRequest) (*models.User, error) {
	// 检查用户名是否已存在
	var existingUser models.User
	if err := s.db.Where("username = ? OR email = ?", req.Username, req.Email).First(&existingUser).Error; err == nil {
		return nil, errors.New("用户名或邮箱已存在")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	// 创建用户
	user := &models.User{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
		Role:     models.RoleUser,
		IsActive: true,
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	return user, nil
}

// Login 用户登录
func (s *AuthService) Login(req *models.LoginRequest) (*models.LoginResponse, error) {
	// 查找用户
	var user models.User
	if err := s.db.Where("username = ? OR email = ?", req.Username, req.Username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 检查用户是否激活
	if !user.IsActive {
		return nil, errors.New("用户账户已被禁用")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 生成JWT token
	token, expiresAt, err := s.GenerateToken(&user)
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	// 更新最后登录时间
	now := time.Now()
	user.LastLogin = &now
	s.db.Save(&user)

	return &models.LoginResponse{
		Token:     token,
		User:      user,
		ExpiresAt: expiresAt,
	}, nil
}

// GenerateToken 生成JWT token
func (s *AuthService) GenerateToken(user *models.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.tokenTTL)

	claims := &JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", user.ID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ValidateToken 验证JWT token
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("意外的签名方法: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的token")
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(userID uint) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return &user, nil
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID uint, req *models.ChangePasswordRequest) error {
	// 获取用户
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return errors.New("原密码错误")
	}

	// 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %w", err)
	}

	// 更新密码
	if err := s.db.Model(user).Update("password", string(hashedPassword)).Error; err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	return nil
}

// GetUserProfile 获取用户资料
func (s *AuthService) GetUserProfile(userID uint) (*models.UserProfile, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	return &models.UserProfile{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt,
		LastLogin: user.LastLogin,
	}, nil
}

// UpdateProfile 更新用户资料
func (s *AuthService) UpdateProfile(userID uint, req *models.UserProfile) error {
	// 获取用户
	user, err := s.GetUserByID(userID)
	if err != nil {
		return err
	}

	// 检查邮箱是否已被其他用户使用
	if req.Email != user.Email {
		var existingUser models.User
		if err := s.db.Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			return errors.New("邮箱已被其他用户使用")
		}
	}

	// 更新用户信息
	updates := map[string]interface{}{
		"email": req.Email,
	}

	if err := s.db.Model(user).Updates(updates).Error; err != nil {
		return fmt.Errorf("更新用户资料失败: %w", err)
	}

	return nil
}

// RevokeToken 撤销token（简单实现，实际应该使用Redis等缓存）
func (s *AuthService) RevokeToken(tokenString string) error {
	// 这里是一个简单的实现，实际生产环境中应该使用Redis等缓存来存储被撤销的token
	// 由于这是一个演示项目，我们只是验证token的有效性
	_, err := s.ValidateToken(tokenString)
	if err != nil {
		return errors.New("无效的token")
	}

	// 在实际实现中，这里应该将token添加到黑名单
	// 例如：redis.Set("blacklist:"+tokenString, true, tokenTTL)
	
	return nil
}

// RefreshToken 刷新token
func (s *AuthService) RefreshToken(userID uint) (*models.LoginResponse, error) {
	// 获取用户
	user, err := s.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 检查用户是否激活
	if !user.IsActive {
		return nil, errors.New("用户账户已被禁用")
	}

	// 生成新的JWT token
	token, expiresAt, err := s.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	return &models.LoginResponse{
		Token:     token,
		User:      *user,
		ExpiresAt: expiresAt,
	}, nil
}