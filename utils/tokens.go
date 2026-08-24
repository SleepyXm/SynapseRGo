package utils

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

// --- JWT ---

func CreateAccessToken(userID string) (string, error) {
	claims := Claims{
		Sub: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(
				time.Duration(Cfg.AccessTokenExpireMinutes) * time.Minute,
			)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(Cfg.SecretKey))
}

func CreateRefreshToken(userID string) (string, error) {
	claims := Claims{
		Sub:  userID,
		Type: "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(
				time.Duration(Cfg.RefreshTokenExpireDays) * 24 * time.Hour,
			)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(Cfg.SecretKey))
}

// DecodeAccessToken validates the access-token cookie format and rejects every
// non-access token type, including refresh tokens signed with the same key.
func DecodeAccessToken(cookieValue string) (string, error) {
	if !strings.HasPrefix(cookieValue, "Bearer ") {
		return "", fmt.Errorf("invalid access token format")
	}
	tokenStr := strings.TrimPrefix(cookieValue, "Bearer ")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(Cfg.SecretKey), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || claims.Sub == "" || claims.Type != "" {
		return "", fmt.Errorf("invalid claims")
	}
	return claims.Sub, nil
}

func DecodeRefreshToken(tokenStr string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(Cfg.SecretKey), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid or expired refresh token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || claims.Type != "refresh" || claims.Sub == "" {
		return "", fmt.Errorf("invalid token type")
	}
	return claims.Sub, nil
}

// --- Redis refresh token store ---

func StoreRefreshToken(ctx context.Context, userID, token string) error {
	key := fmt.Sprintf("refresh:%s", token)
	ttl := time.Duration(Cfg.RefreshTokenExpireDays) * 24 * time.Hour
	return RDB.SetEx(ctx, key, userID, ttl).Err()
}

func GetStoredRefreshToken(ctx context.Context, token string) (string, error) {
	key := fmt.Sprintf("refresh:%s", token)
	val, err := RDB.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", fmt.Errorf("refresh token invalid or expired")
	}
	return val, err
}

func RevokeRefreshToken(ctx context.Context, token string) error {
	return RDB.Del(ctx, fmt.Sprintf("refresh:%s", token)).Err()
}

// --- Cookies ---

func cookiePolicy(c *gin.Context) (http.SameSite, bool) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	if secure {
		return http.SameSiteNoneMode, true
	}
	return http.SameSiteLaxMode, false
}

func SetAuthCookies(c *gin.Context, accessToken, refreshToken string) {
	sameSite, secure := cookiePolicy(c)
	c.SetSameSite(sameSite)
	c.SetCookie(
		"access_token",
		"Bearer "+accessToken,
		60*Cfg.AccessTokenExpireMinutes,
		"/",
		"",
		secure,
		true,
	)
	c.SetCookie(
		"refresh_token",
		refreshToken,
		60*60*24*Cfg.RefreshTokenExpireDays,
		"/",
		"",
		secure,
		true,
	)
}

func ClearAuthCookies(c *gin.Context) {
	sameSite, secure := cookiePolicy(c)
	c.SetSameSite(sameSite)
	c.SetCookie("access_token", "", -1, "/", "", secure, true)
	c.SetCookie("refresh_token", "", -1, "/", "", secure, true)
}
