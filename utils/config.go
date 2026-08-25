package utils

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

var Cfg Config

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	Cfg = Config{
		DatabaseURL:              requireEnv("DATABASE"),
		SecretKey:                requireEnv("SECRET_KEY"),
		Algorithm:                requireEnv("ALGORITHM"),
		AccessTokenExpireMinutes: requireEnvInt("ACCESS_TOKEN_EXPIRE_MINUTES"),
		RefreshTokenExpireDays:   requireEnvInt("REFRESH_TOKEN_EXPIRE_DAYS"),
		EncryptionKey:            requireEnv("ENCRYPTION_KEY"),
		DevServer:                requireEnv("DEV_SERVER"),
		DevServerBackend:         requireEnv("DEV_SERVER_BACKEND"),
		FrontendProd:             requireEnv("FRONTEND_PROD"),
		ResendAPIKey:             requireEnv("RESEND_API_KEY"),
		RedisAddr:                requireEnv("REDIS_URL"),
		RAGObjectRoot:            envOrDefault("RAG_OBJECT_ROOT", "./knowledge"),
		RAGWorkerPollSeconds:     envIntOrDefault("RAG_WORKER_POLL_SECONDS", 2),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		log.Fatalf("Env var %s must be a positive integer, got: %s", key, value)
	}
	return n
}

func requireEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Required env var %s is not set", key)
	}
	return val
}

func requireEnvInt(key string) int {
	val := requireEnv(key)
	n, err := strconv.Atoi(val)
	if err != nil {
		log.Fatalf("Env var %s must be an integer, got: %s", key, val)
	}
	return n
}
