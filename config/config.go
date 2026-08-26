package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address          string
	DataDir          string
	LogLevel         string
	DispatchInterval time.Duration
	SnapshotInterval time.Duration
	ShutdownTimeout  time.Duration
}

func Default() Config {
	return Config{Address: "127.0.0.1:8080", DataDir: "./data", LogLevel: "info", DispatchInterval: 250 * time.Millisecond, SnapshotInterval: 30 * time.Second, ShutdownTimeout: 10 * time.Second}
}

func Load() (Config, error) {
	value := Default()
	if text := os.Getenv("EDGECONFIG_ADDRESS"); text != "" {
		value.Address = text
	}
	if text := os.Getenv("EDGECONFIG_DATA_DIR"); text != "" {
		value.DataDir = text
	}
	if text := os.Getenv("EDGECONFIG_LOG_LEVEL"); text != "" {
		value.LogLevel = strings.ToLower(text)
	}
	var err error
	if value.DispatchInterval, err = durationEnv("EDGECONFIG_DISPATCH_INTERVAL", value.DispatchInterval); err != nil {
		return Config{}, err
	}
	if value.SnapshotInterval, err = durationEnv("EDGECONFIG_SNAPSHOT_INTERVAL", value.SnapshotInterval); err != nil {
		return Config{}, err
	}
	if value.ShutdownTimeout, err = durationEnv("EDGECONFIG_SHUTDOWN_TIMEOUT", value.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if value.Address == "" || value.DataDir == "" {
		return Config{}, fmt.Errorf("address and data directory are required")
	}
	return value, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	text := os.Getenv(name)
	if text == "" {
		return fallback, nil
	}
	if seconds, err := strconv.Atoi(text); err == nil {
		if seconds <= 0 {
			return 0, fmt.Errorf("%s must be positive", name)
		}
		return time.Duration(seconds) * time.Second, nil
	}
	value, err := time.ParseDuration(text)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return value, nil
}
