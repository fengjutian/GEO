package config

import (
	"bufio"
	"os"
	"strings"
)

type Config struct {
	Port          string
	DBDSN         string
	GINMode       string
	AllowedOrigin map[string]bool
}

func Load() Config {
	LoadDotEnv(".env")
	return Config{
		Port:          getenv("PORT", "8080"),
		DBDSN:         getenv("DB_DSN", "geopilot:change_me_geopilot@tcp(127.0.0.1:3306)/geopilot?charset=utf8mb4&parseTime=True&loc=Local"),
		GINMode:       getenv("GIN_MODE", ""),
		AllowedOrigin: allowedOrigins(getenv("CORS_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173,http://localhost:4173,http://127.0.0.1:4173")),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func allowedOrigins(value string) map[string]bool {
	origins := make(map[string]bool)
	for _, origin := range strings.Split(value, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = true
		}
	}
	return origins
}

// LoadDotEnv provides a dependency-free way to use the documented .env file.
// Real environment variables always win over values in the file.
func LoadDotEnv(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}
