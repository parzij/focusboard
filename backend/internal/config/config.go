package config

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strconv"
	"time"
)

type Database struct {
	URL            string
	ConnectTimeout time.Duration
	CheckTimeout   time.Duration
	MaxConns       int32
	MinConns       int32
	MaxConnIdle    time.Duration
	MaxConnAge     time.Duration
}

type Config struct {
	Database   Database
	HTTPAddr   string
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
	Shutdown   time.Duration
	LogLevel   slog.Level
}

func Load() (Config, error) {
	return load(os.Getenv)
}

func load(get func(string) string) (Config, error) {
	var cfg Config
	db, err := loadDatabase(get)
	if err != nil {
		return cfg, err
	}
	cfg.Database = db
	cfg.HTTPAddr = get("HTTP_ADDR")
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = "127.0.0.1:8080"
	}
	_, port, err := net.SplitHostPort(cfg.HTTPAddr)
	number, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || number == 0 {
		return cfg, fmt.Errorf("HTTP_ADDR must be a host and numeric port (1–65535)")
	}
	if level := get("LOG_LEVEL"); level != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(level)); err != nil {
			return cfg, fmt.Errorf("LOG_LEVEL must be a valid slog level")
		}
	}
	if err := readDurations(get, []durationSetting{
		{"HTTP_READ_HEADER_TIMEOUT", 5 * time.Second, &cfg.ReadHeader},
		{"HTTP_READ_TIMEOUT", 10 * time.Second, &cfg.Read},
		{"HTTP_WRITE_TIMEOUT", 15 * time.Second, &cfg.Write},
		{"HTTP_IDLE_TIMEOUT", time.Minute, &cfg.Idle},
		{"HTTP_SHUTDOWN_TIMEOUT", 5 * time.Second, &cfg.Shutdown},
	}); err != nil {
		return cfg, err
	}
	if cfg.ReadHeader > cfg.Read {
		return cfg, fmt.Errorf("HTTP_READ_HEADER_TIMEOUT must not exceed HTTP_READ_TIMEOUT")
	}
	if db.CheckTimeout >= cfg.Write {
		return cfg, fmt.Errorf("DB_CHECK_TIMEOUT must be shorter than HTTP_WRITE_TIMEOUT")
	}
	return cfg, nil
}

func LoadDatabase() (Database, error) {
	return loadDatabase(os.Getenv)
}

func loadDatabase(get func(string) string) (Database, error) {
	db := Database{URL: get("DATABASE_URL"), MaxConns: 10}
	if db.URL == "" {
		return db, fmt.Errorf("DATABASE_URL is required")
	}
	if err := readDurations(get, []durationSetting{
		{"DB_CONNECT_TIMEOUT", 3 * time.Second, &db.ConnectTimeout},
		{"DB_CHECK_TIMEOUT", time.Second, &db.CheckTimeout},
		{"DB_MAX_CONN_IDLE_TIME", 5 * time.Minute, &db.MaxConnIdle},
		{"DB_MAX_CONN_LIFETIME", time.Hour, &db.MaxConnAge},
	}); err != nil {
		return db, err
	}
	for key, target := range map[string]*int32{
		"DB_POOL_MAX_CONNS": &db.MaxConns,
		"DB_POOL_MIN_CONNS": &db.MinConns,
	} {
		if raw := get(key); raw != "" {
			value, err := strconv.ParseUint(raw, 10, 31)
			if err != nil {
				return db, fmt.Errorf("%s must be a non-negative 32-bit integer", key)
			}
			*target = int32(value)
		}
	}
	if db.MaxConns == 0 || db.MinConns > db.MaxConns {
		return db, fmt.Errorf("DB_POOL_MAX_CONNS must be positive and at least DB_POOL_MIN_CONNS")
	}
	return db, nil
}

type durationSetting struct {
	key          string
	defaultValue time.Duration
	target       *time.Duration
}

func readDurations(get func(string) string, settings []durationSetting) error {
	for _, setting := range settings {
		raw := get(setting.key)
		if raw == "" {
			*setting.target = setting.defaultValue
			continue
		}
		value, err := time.ParseDuration(raw)
		if err != nil || value <= 0 {
			return fmt.Errorf("%s must be a positive duration", setting.key)
		}
		*setting.target = value
	}
	return nil
}
