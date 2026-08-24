package utils

import (
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
)

// Fernet equivalent — AES-GCM via standard library is the Go idiom,
// but since you're porting from Fernet we'll use a direct AES-GCM wrapper
// that's compatible with the same ENCRYPTION_KEY pattern.

// DummyPasswordHash is used to prevent timing attacks when a user is not found.
// Pre-computed bcrypt hash so verify_password always does real work.
const DummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func VerifyPassword(plain, hashed string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain)) == nil
}

// --- Refresh token store (Redis) ---

var RDB *redis.Client

func InitRedis() {
	RDB = redis.NewClient(&redis.Options{
		Addr:     Cfg.RedisAddr,
		PoolSize: 1000,
	})
}
