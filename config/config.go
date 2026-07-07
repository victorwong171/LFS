package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// Config represents the application configuration.
type Config struct {
	StoragePath string `json:"storage_path"` // File storage path
	EnableMD5   bool   `json:"enable_md5"`    // Enable MD5 validation
}

// LoadConfig loads configuration from a configuration file or environment variables.
// Prioritizes config.json, then environment variables, then defaults.
func LoadConfig() Config {
	cfg := Config{
		EnableMD5: true, // Default to true
	}

	// 1. Try to load from config.json in the same directory as main.go first, then fallback to working directory
	configFile := "cmd/lfs-server/config.json"
	data, err := os.ReadFile(configFile)
	if err != nil {
		configFile = "config.json"
		data, err = os.ReadFile(configFile)
	}

	if err == nil {
		if err := json.Unmarshal(data, &cfg); err == nil {
			fmt.Printf("Loaded configuration from %s\n", configFile)
		} else {
			fmt.Printf("Failed to parse %s: %v\n", configFile, err)
		}
	}

	// 2. Override with environment variables if present
	if envStoragePath := os.Getenv("LFS_STORAGE_PATH"); envStoragePath != "" {
		cfg.StoragePath = envStoragePath
	}
	if envEnableMD5 := os.Getenv("LFS_ENABLE_MD5"); envEnableMD5 != "" {
		if val, err := strconv.ParseBool(envEnableMD5); err == nil {
			cfg.EnableMD5 = val
		}
	}

	// 3. Apply defaults if still empty
	if cfg.StoragePath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfg.StoragePath = filepath.Join(home, "Downloads")
		} else {
			cfg.StoragePath = "./data"
		}
		fmt.Printf("STORAGE_PATH not set, using default: %s\n", cfg.StoragePath)
	}

	return cfg
}
