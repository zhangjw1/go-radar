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

// Settings 保存 Go Radar 启动时解析出的运行配置。
type Settings struct {
	AppName         string // AppName 是页面和健康检查中展示的应用名称。
	DatabaseURL     string // DatabaseURL 是 .env 或环境变量中的原始数据库连接串。
	DatabasePath    string // DatabasePath 是解析后的 SQLite 文件路径，供 sqlite driver 直接打开。
	EnvPath         string // EnvPath 是实际加载的 .env 文件路径；为空表示未加载 .env。
	EnvDir          string // EnvDir 是 .env 所在目录，用于解析 sqlite:///./radar.db 这类相对路径。
	Port            string // Port 是 Go 服务监听端口，默认 8080，避免占用 Python 服务端口。
	EnableScheduler bool   // EnableScheduler 控制 Go 版调度器是否真正运行扫描任务。
	AutoMigrate     bool   // AutoMigrate 控制是否仅为缺失表创建 schema，不会修改已有 Python 表。
}

// Load 从环境变量和 .env 文件读取配置，并把 SQLite URL 解析成可打开的本地路径。
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

	databaseURL := firstNonEmpty(os.Getenv("DATABASE_URL"), "sqlite:///./radar.db")
	databasePath, err := SQLitePathFromURL(databaseURL, envDir)
	if err != nil {
		return nil, err
	}

	return &Settings{
		AppName:         firstNonEmpty(os.Getenv("APP_NAME"), "Web3 Online Radar"),
		DatabaseURL:     databaseURL,
		DatabasePath:    databasePath,
		EnvPath:         envPath,
		EnvDir:          envDir,
		Port:            firstNonEmpty(os.Getenv("GO_RADAR_PORT"), defaultPort),
		EnableScheduler: envBool("GO_RADAR_ENABLE_SCHEDULER", false),
		AutoMigrate:     envBool("GO_RADAR_AUTO_MIGRATE", true),
	}, nil
}

// SQLitePathFromURL 将 sqlite:/// 开头的 DATABASE_URL 转为 SQLite 文件路径。
func SQLitePathFromURL(databaseURL string, baseDir string) (string, error) {
	const prefix = "sqlite:///"
	if !strings.HasPrefix(databaseURL, prefix) {
		return "", fmt.Errorf("only sqlite database URLs are supported, got %q", databaseURL)
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

// findEnvFile 按 GO_RADAR_ENV_FILE、当前目录、父目录的顺序查找 .env。
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

// readEnvFile 读取简单的 KEY=VALUE 格式 .env，并忽略空行和注释行。
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

// firstNonEmpty 返回第一项非空字符串，常用于环境变量默认值兜底。
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

// envBool 按常见布尔写法解析环境变量，无法解析或为空时返回 fallback。
func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	return raw == "1" || strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes") || strings.EqualFold(raw, "on")
}
