package config

import (
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server    ServerConfig
	Database  DatabaseConfig
	JWT       JWTConfig
	Anthropic AnthropicConfig
	WeChat    WeChatConfig
}

type ServerConfig struct {
	Port string
	Env  string
}

type DatabaseConfig struct {
	Path string
}

type JWTConfig struct {
	Secret      string
	ExpireHours int
}

type AnthropicConfig struct {
	APIKey string
	Model  string
}

type WeChatConfig struct {
	AppID       string
	AppSecret   string
	RedirectURI string
}

var AppConfig *Config

func LoadConfig() *Config {
	godotenv.Load()

	port := getEnv("PORT", "8080")
	env := getEnv("ENV", "development")
	dbPath := getEnv("DB_PATH", "./data/data.db")
	jwtSecret := getEnv("JWT_SECRET", "kegenbao-secret-key")
	jwtExpireHours := getEnvInt("JWT_EXPIRE_HOURS", 720)

	// AI config
	anthropicAPIKey := getEnv("ANTHROPIC_API_KEY", "")
	openAIEndpoint := getEnv("OPENAI_API_ENDPOINT", "")
	openAIKey := getEnv("OPENAI_API_KEY", "")
	aiModel := getEnv("AI_MODEL", "minimax")

	// WeChat config
	wechatAppID := getEnv("WECHAT_APP_ID", "")
	wechatAppSecret := getEnv("WECHAT_APP_SECRET", "")
	wechatRedirectURI := getEnv("WECHAT_REDIRECT_URI", "")

	AppConfig = &Config{
		Server: ServerConfig{
			Port: port,
			Env:  env,
		},
		Database: DatabaseConfig{
			Path: dbPath,
		},
		JWT: JWTConfig{
			Secret:      jwtSecret,
			ExpireHours: jwtExpireHours,
		},
		Anthropic: AnthropicConfig{
			APIKey: anthropicAPIKey,
			Model:  "claude-sonnet-4-20250514",
		},
		WeChat: WeChatConfig{
			AppID:       wechatAppID,
			AppSecret:   wechatAppSecret,
			RedirectURI: wechatRedirectURI,
		},
	}

	// Store OpenAI config in environment for handlers to use
	if openAIEndpoint != "" {
		os.Setenv("OPENAI_API_ENDPOINT", openAIEndpoint)
	}
	if openAIKey != "" {
		os.Setenv("OPENAI_API_KEY", openAIKey)
	}
	if aiModel != "" {
		os.Setenv("AI_MODEL", aiModel)
	}

	return AppConfig
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func (c *Config) GetJWTExpiration() time.Time {
	return time.Now().Add(time.Duration(c.JWT.ExpireHours) * time.Hour)
}