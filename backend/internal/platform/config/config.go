package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	UploadsDir     string
	DatabaseURL    string
	MaxUploadBytes int64
}

func Load() Config {
	loadDotEnvFiles()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "./uploads"
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:postgres@localhost:5432/cs2_demos?sslmode=disable"
	}

	maxUploadBytes := int64(1024 * 1024 * 1024)
	maxUploadBytesEnv := os.Getenv("MAX_UPLOAD_BYTES")
	if maxUploadBytesEnv != "" {
		parsed, err := strconv.ParseInt(maxUploadBytesEnv, 10, 64)
		if err == nil && parsed > 0 {
			maxUploadBytes = parsed
		}
	}

	return Config{
		Port:           port,
		UploadsDir:     uploadsDir,
		DatabaseURL:    databaseURL,
		MaxUploadBytes: maxUploadBytes,
	}
}

func loadDotEnvFiles() {
	values := map[string]string{}

	mergeEnvFile(values, ".env")
	mergeEnvFile(values, ".env.local")
	mergeEnvFile(values, "../.env")
	mergeEnvFile(values, "../.env.local")

	for key, value := range values {
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func mergeEnvFile(target map[string]string, filePath string) {
	loaded, err := godotenv.Read(filePath)
	if err != nil {
		return
	}

	for key, value := range loaded {
		target[key] = value
	}
}
