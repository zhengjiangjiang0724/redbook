package service

import (
	"errors"
	"redbook/config"
	"redbook/dao"
	"redbook/internal/auth"
	"redbook/model"
	"redbook/utils"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrUserExists = errors.New("user already exists")

// UserService bundles the DAO, session storage and authentication helpers.
type UserService struct {
	dao     *dao.UserDAO
	Session *auth.SessionManager // 使用 internal/auth 中的 SessionManager
}

// NewUserService 创建一个新的 UserService 实例
func NewUserService(dao *dao.UserDAO, rdb *redis.Client) *UserService {
	return &UserService{
		dao:     dao,
		Session: auth.NewSessionManager(rdb), // 初始化 auth.SessionManager
	}
}

// Register persists a freshly created user after hashing the password.
func (s *UserService) Register(user *model.User) error {
	hashed, err := utils.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashed
	if err := s.dao.CreateUser(user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrUserExists
		}
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrUserExists
		}
		return err
	}
	return nil
}

// Login handles username/password authentication and issues a token pair.
func (s *UserService) Login(username, password, device string) (string, string, error) {
	user, err := s.dao.GetByUsername(username)
	if err != nil || user.ID == 0 {
		return "", "", errors.New("用户名或密码错误")
	}

	// 校验密码
	if !utils.CheckPasswordHash(password, user.Password) {
		return "", "", errors.New("用户名或密码错误")
	}

	// 使用 SessionManager 存储 Refresh Token 和生成 Token
	accessToken, refreshToken, err := auth.GenerateTokens(uint(user.ID), device)
	if err != nil {
		return "", "", err
	}

	// 保存 Refresh Token 到 Redis
	ttl := time.Duration(config.GlobalConfig.JWT.RefreshExpire) * time.Second
	s.Session.SaveRefreshToken(uint(user.ID), device, refreshToken, ttl)

	// 返回生成的 Access Token 和 Refresh Token
	return accessToken, refreshToken, nil
}

// RotateRefreshToken 校验 refresh token、执行黑名单写入，并颁发新的 token 对。
func (s *UserService) RotateRefreshToken(refreshToken, headerDevice string) (string, string, error) {
	if refreshToken == "" {
		return "", "", errors.New("missing refresh token")
	}

	claims, err := auth.ParseToken(refreshToken)
	if err != nil {
		return "", "", errors.New("refresh token invalid")
	}

	// 可选：若客户端提供 X-Device，需与 Token claims 匹配。
	if headerDevice != "" && headerDevice != claims.Device {
		return "", "", errors.New("device mismatch")
	}

	stored, err := s.Session.GetRefreshToken(claims.UserID, claims.Device)
	if err != nil || stored != refreshToken {
		return "", "", errors.New("refresh token expired or rotated")
	}

	accessToken, newRefresh, err := auth.GenerateTokens(claims.UserID, claims.Device)
	if err != nil {
		return "", "", err
	}

	ttl := time.Duration(config.GlobalConfig.JWT.RefreshExpire) * time.Second
	if err := s.Session.SaveRefreshToken(claims.UserID, claims.Device, newRefresh, ttl); err != nil {
		return "", "", err
	}

	// 将旧 refresh token 加入黑名单，防止被重放。
	_ = s.Session.AddBlackList(refreshToken, ttl)

	return accessToken, newRefresh, nil
}
