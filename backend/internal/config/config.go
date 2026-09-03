package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port                        string
	Host                        string
	BaseURL                     string
	AppEnv                      string
	DBPath                      string
	JWTSecret                   string
	AnonymousDailyQuota         int
	RegisteredMonthlyQuota      int
	AnonymousMaxExpirationDays  int
	RegisteredMaxExpirationDays int
	TurnstileEnabled            bool
	TurnstileSiteKey            string
	TurnstileSecretKey          string
	AdminBootstrapEmail         string
	AdminBootstrapPassword      string
	FirebaseAPIKey              string
	FirebaseAuthDomain          string
	FirebaseProjectID           string
	FirebaseStorageBucket       string
	FirebaseMessagingSenderID   string
	FirebaseAppID               string
}

// Load reads configuration from environment variables, optionally reading a .env file first.
func Load(envFilePath string) *Config {
	loadDotEnv(envFilePath)

	cfg := &Config{
		Port:                        getEnv("PORT", "3000"),
		Host:                        getEnv("HOST", "127.0.0.1"),
		BaseURL:                     getEnv("BASE_URL", "http://localhost:3000"),
		AppEnv:                      getEnv("APP_ENV", "development"),
		DBPath:                      getEnv("DB_PATH", "./data/go-dev.sqlite"),
		JWTSecret:                   getEnv("JWT_SECRET", "change-me-super-secret-default-key-32-chars!"),
		AnonymousDailyQuota:         getEnvInt("ANONYMOUS_DAILY_QUOTA", 15),
		RegisteredMonthlyQuota:      getEnvInt("REGISTERED_MONTHLY_QUOTA", 100),
		AnonymousMaxExpirationDays:  getEnvInt("ANONYMOUS_MAX_EXPIRATION_DAYS", 7),
		RegisteredMaxExpirationDays: getEnvInt("REGISTERED_MAX_EXPIRATION_DAYS", 365),
		TurnstileEnabled:            getEnvBool("TURNSTILE_ENABLED", false),
		TurnstileSiteKey:            getEnv("TURNSTILE_SITE_KEY", ""),
		TurnstileSecretKey:          getEnv("TURNSTILE_SECRET_KEY", ""),
		AdminBootstrapEmail:         getEnv("ADMIN_BOOTSTRAP_EMAIL", "admin@example.com"),
		AdminBootstrapPassword:      getEnv("ADMIN_BOOTSTRAP_PASSWORD", ""),
		FirebaseAPIKey:              getEnv("FIREBASE_API_KEY", ""),
		FirebaseAuthDomain:          getEnv("FIREBASE_AUTH_DOMAIN", ""),
		FirebaseProjectID:           getEnv("FIREBASE_PROJECT_ID", ""),
		FirebaseStorageBucket:       getEnv("FIREBASE_STORAGE_BUCKET", ""),
		FirebaseMessagingSenderID:   getEnv("FIREBASE_MESSAGING_SENDER_ID", ""),
		FirebaseAppID:               getEnv("FIREBASE_APP_ID", ""),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	v, err := strconv.Atoi(valStr)
	if err != nil {
		return fallback
	}
	return v
}

func getEnvBool(key string, fallback bool) bool {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	b, err := strconv.ParseBool(valStr)
	if err != nil {
		return fallback
	}
	return b
}

func loadDotEnv(filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		return // File does not exist, ignore
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}
