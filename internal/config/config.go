package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultPort = "8080"

type DBDriver string

const (
	DBDriverSQLite   DBDriver = "sqlite"
	DBDriverPostgres DBDriver = "postgres"
)

// Settings 保存 Go Radar 启动时解析出的运行配置。
type Settings struct {
	AppName         string
	DatabaseURL     string
	DatabaseDriver  DBDriver
	DatabasePath    string
	EnvPath         string
	EnvDir          string
	Port            string
	EnableScheduler bool
	AutoMigrate     bool
}

// Load 从环境变量和 .env 文件读取配置，并解析数据库连接。
func Load() (*Settings, error) {
	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	envPath, err := findEnvFile()
	if err != nil {
		return nil, err
	}
	envDir := workingDir
	if envPath != "" {
		envValues, err := readEnvFile(envPath)
		if err != nil {
			return nil, err
		}
		for key, value := range envValues {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
		envDir = filepath.Dir(envPath)
	}

	databaseURL := firstNonEmpty(
		os.Getenv("DATABASE_URL"),
		"sqlite:///./radar.db",
	)
	driver, databasePath, err := ParseDatabaseURL(databaseURL, envDir)
	if err != nil {
		return nil, err
	}

	return &Settings{
		AppName:         firstNonEmpty(os.Getenv("APP_NAME"), "Web3 Online Radar"),
		DatabaseURL:     databaseURL,
		DatabaseDriver:  driver,
		DatabasePath:    databasePath,
		EnvPath:         envPath,
		EnvDir:          envDir,
		Port:            firstNonEmpty(os.Getenv("GO_RADAR_PORT"), defaultPort),
		EnableScheduler: envBool("GO_RADAR_ENABLE_SCHEDULER", false),
		AutoMigrate:     envBool("GO_RADAR_AUTO_MIGRATE", true),
	}, nil
}

// ParseDatabaseURL 解析数据库 URL；SQLite 返回文件路径，PostgreSQL 返回原始连接串。
func ParseDatabaseURL(databaseURL string, baseDir string) (DBDriver, string, error) {
	if strings.HasPrefix(databaseURL, "sqlite:///") {
		path, err := SQLitePathFromURL(databaseURL, baseDir)
		return DBDriverSQLite, path, err
	}
	if strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://") {
		return DBDriverPostgres, strings.TrimSpace(databaseURL), nil
	}
	return "", "", fmt.Errorf("unsupported DATABASE_URL %q", databaseURL)
}

// SQLitePathFromURL 将 sqlite:/// 开头的 DATABASE_URL 转为 SQLite 文件路径。
func SQLitePathFromURL(databaseURL string, baseDir string) (string, error) {
	const prefix = "sqlite:///"
	if !strings.HasPrefix(databaseURL, prefix) {
		return "", fmt.Errorf("unsupported sqlite database URL %q", databaseURL)
	}

	rawPath := strings.TrimPrefix(databaseURL, prefix)
	if rawPath == "" {
		return "", errors.New("sqlite database path is empty")
	}
	if rawPath == ":memory:" {
		return rawPath, nil
	}
	if filepath.IsAbs(rawPath) {
		return filepath.Clean(rawPath), nil
	}
	return filepath.Clean(filepath.Join(baseDir, rawPath)), nil
}

func findEnvFile() (string, error) {
	if override := strings.TrimSpace(os.Getenv("GO_RADAR_ENV_FILE")); override != "" {
		if strings.EqualFold(override, "none") || strings.EqualFold(override, "off") || strings.EqualFold(override, "false") {
			return "", nil
		}
		abs, err := filepath.Abs(override)
		if err != nil {
			return "", err
		}
		if info, err := os.Stat(abs); err == nil && !info.IsDir() {
			return abs, nil
		}
		return "", fmt.Errorf("GO_RADAR_ENV_FILE points to a missing file: %s", override)
	}

	start, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := start
	for {
		candidate := filepath.Join(dir, ".env")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Abs(candidate)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", nil
}

func readEnvFile(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		key, value, _ := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			continue
		}
		values[key] = strings.Trim(value, `"'`)
	}
	return values, scanner.Err()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
}
