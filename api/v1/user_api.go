package v1

import (
	"errors"
	"net/http"
	"redbook/api/v1/request"
	"redbook/config"
	"redbook/internal/auth"
	"redbook/internal/metrics"
	"redbook/model"
	"redbook/service"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// UserAPI exposes HTTP handlers for registration/login/logout flows.
// UserAPI 聚合了所有与用户鉴权相关的 HTTP Handler。
type UserAPI struct {
	service *service.UserService
}

// NewUserAPI wires the service layer into the HTTP handlers.
func NewUserAPI(s *service.UserService) *UserAPI {
	return &UserAPI{service: s}
}

// Register handles new account creation.
func (u *UserAPI) Register(c *gin.Context) {
	var req request.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	err := u.service.Register(&model.User{
		Username: req.Username,
		Password: req.Password,
		Mobile:   req.Mobile,
	})
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "user already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "注册成功"})
}

// Login validates user credentials and returns a new token pair.
func (u *UserAPI) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.IncLogin("bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	device := c.GetHeader("X-Device")
	access, refresh, err := u.service.Login(req.Username, req.Password, device)
	if err != nil {
		metrics.IncLogin("unauthorized")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	metrics.IncLogin("success")
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// RefreshToken 验证 refresh token，执行 rotation 并返回新的 token 对。
func (u *UserAPI) RefreshToken(c *gin.Context) {
	var req request.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		metrics.IncRefresh("bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	device := c.GetHeader("X-Device")
	access, refresh, err := u.service.RotateRefreshToken(req.RefreshToken, device)
	if err != nil {
		metrics.IncRefresh("unauthorized")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	metrics.IncRefresh("success")
	c.JSON(http.StatusOK, gin.H{
		"access_token":  access,
		"refresh_token": refresh,
	})
}

// Logout 支持使用 Access Token 或 Refresh Token 注销
func (u *UserAPI) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		metrics.IncLogout("bad_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing token"})
		return
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

	// 情况 1：Authorization 携带 Access Token，直接将其列入黑名单并删除对应 refresh。
	claims, err := auth.ParseToken(tokenStr)
	if err == nil {
		// token 有效 —— treat as access token (可能是未过期的 access)
		// 将 access 加入黑名单，TTL 用 access expire
		if err := u.service.Session.AddBlackList(tokenStr,
			time.Duration(config.GlobalConfig.JWT.AccessExpire)*time.Second); err != nil {
			metrics.IncLogout("internal_error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "blacklist failed"})
			return
		}
		// 删除对应 refresh token 存储
		_ = u.service.Session.DeleteRefreshToken(claims.UserID, claims.Device)

		metrics.IncLogout("success")
		c.JSON(http.StatusOK, gin.H{"message": "logout success"})
		return
	}

	// 情况 2：token 不是 Access（或 access 已失效），改用宽松解析，把它视为 Refresh Token。
	claims, err = auth.ParseTokenAllowExpired(tokenStr)
	if err != nil {
		metrics.IncLogout("invalid_token")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	// 验证此 refresh 与 Redis 中存的是否一致
	stored, err := u.service.Session.GetRefreshToken(claims.UserID, claims.Device)
	if err != nil || stored == "" || stored != tokenStr {
		metrics.IncLogout("refresh_mismatch")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "refresh invalid or expired"})
		return
	}

	// 把 refresh 加入黑名单（防止重放）
	if err := u.service.Session.AddBlackList(tokenStr,
		time.Duration(config.GlobalConfig.JWT.RefreshExpire)*time.Second); err != nil {
		metrics.IncLogout("internal_error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "blacklist failed"})
		return
	}

	// 删除 Redis 中的 refresh 记录
	if err := u.service.Session.DeleteRefreshToken(claims.UserID, claims.Device); err != nil {
		// log 但仍视为成功
	}

	metrics.IncLogout("success")
	c.JSON(http.StatusOK, gin.H{"message": "logout success"})
}
