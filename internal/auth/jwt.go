package auth

import (
	"errors"
	"time"

	"redbook/config"

	"github.com/golang-jwt/jwt/v5"
)

// Claims defines the JWT payload shared by both access and refresh tokens.
// It embeds RegisteredClaims so expiration and issuance metadata are centralized.
type Claims struct {
	UserID uint   `json:"user_id"`
	Device string `json:"device"`
	jwt.RegisteredClaims
}

// GenerateTokens issues a short-lived access token and a longer-lived refresh token
// for the given user / device pair. Both tokens share the same claim structure.
func GenerateTokens(userID uint, device string) (accessToken, refreshToken string, err error) {
	now := time.Now()
	accessClaims := Claims{
		UserID: userID,
		Device: device,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(config.GlobalConfig.JWT.AccessExpire) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
	if err != nil {
		return "", "", err
	}

	refreshClaims := Claims{
		UserID: userID,
		Device: device,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(config.GlobalConfig.JWT.RefreshExpire) * time.Second)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString([]byte(config.GlobalConfig.JWT.Secret))
	return
}

// ParseToken validates signature + expiry for standard access usage.
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GlobalConfig.JWT.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

// ParseTokenAllowExpired: 校验签名但允许过期（只用于提取 claims 以支持用 refresh 注销）
// ParseTokenAllowExpired validates the signature but skips expiration checks.
// It is helpful when we need to inspect a refresh token that might have expired.
func ParseTokenAllowExpired(tokenStr string) (*Claims, error) {
	// 使用 WithoutClaimsValidation 以便在验证签名的同时允许过期
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(config.GlobalConfig.JWT.Secret), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
