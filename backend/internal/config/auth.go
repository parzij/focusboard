package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Auth struct {
	Secret                     []byte
	Issuer, Audience, Origin   string
	CookieName                 string
	CookieSecure               bool
	AccessTTL, RefreshTTL      time.Duration
	RequestTimeout, RateWindow time.Duration
	IPLimit, AccountLimit      int
	RateKeys, PasswordWorkers  int
}

func loadAuth(get func(string) string, writeTimeout time.Duration) (Auth, error) {
	cfg := Auth{
		Issuer: get("JWT_ISSUER"), Audience: get("JWT_AUDIENCE"), Origin: get("APP_ORIGIN"),
		CookieName: "focusboard_refresh", CookieSecure: true,
	}
	env := get("APP_ENV")
	if env != "" && env != "development" && env != "production" && env != "test" {
		return cfg, errors.New("APP_ENV must be development, production, or test")
	}
	key, err := base64.StdEncoding.Strict().DecodeString(get("JWT_SECRET"))
	var distinct [256]bool
	for _, b := range key {
		distinct[b] = true
	}
	unique := 0
	for _, present := range distinct {
		if present {
			unique++
		}
	}
	if err != nil || len(key) < 32 || len(key) > 128 || unique < 16 ||
		base64.StdEncoding.EncodeToString(key) != get("JWT_SECRET") {
		return cfg, errors.New("JWT_SECRET must be standard base64 encoding of 32–128 cryptographically random bytes")
	}
	cfg.Secret = key
	for name, value := range map[string]string{"JWT_ISSUER": cfg.Issuer, "JWT_AUDIENCE": cfg.Audience} {
		if value == "" || len(value) > 256 || strings.ContainsAny(value, " \t\r\n") {
			return cfg, fmt.Errorf("%s must be a nonempty identifier of up to 256 bytes", name)
		}
	}
	origin, err := url.Parse(cfg.Origin)
	if err != nil || origin.User != nil || origin.Hostname() == "" ||
		cfg.Origin != origin.Scheme+"://"+origin.Host || (origin.Scheme != "https" && origin.Scheme != "http") {
		return cfg, errors.New("APP_ORIGIN must be an exact HTTP(S) origin without a path")
	}
	if port := origin.Port(); port != "" {
		number, portErr := strconv.ParseUint(port, 10, 16)
		if portErr != nil || number == 0 || (number == 443 && origin.Scheme == "https") || (number == 80 && origin.Scheme == "http") ||
			strconv.FormatUint(number, 10) != port {
			return cfg, errors.New("APP_ORIGIN must use a valid non-default port or omit the port")
		}
	}
	if origin.Host != strings.ToLower(origin.Host) || strings.HasSuffix(origin.Host, ":") {
		return cfg, errors.New("APP_ORIGIN must use a lowercase canonical host")
	}
	if raw := get("COOKIE_SECURE"); raw != "" {
		cfg.CookieSecure, err = strconv.ParseBool(raw)
		if err != nil {
			return cfg, errors.New("COOKIE_SECURE must be boolean")
		}
	}
	if origin.Scheme == "http" && (env != "development" && env != "test" || !isLoopback(origin.Hostname()) || cfg.CookieSecure) {
		return cfg, errors.New("HTTP APP_ORIGIN requires a loopback host and explicit COOKIE_SECURE=false in development or test")
	}
	if !cfg.CookieSecure && (origin.Scheme != "http" || env == "production") {
		return cfg, errors.New("COOKIE_SECURE must be true for HTTPS")
	}
	if raw := get("REFRESH_COOKIE_NAME"); raw != "" {
		if len(raw) > 64 || strings.Trim(raw, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") != "" ||
			strings.HasPrefix(raw, "__Host-") || (strings.HasPrefix(raw, "__Secure-") && !cfg.CookieSecure) {
			return cfg, errors.New("REFRESH_COOKIE_NAME is incompatible with the auth cookie scope")
		}
		cfg.CookieName = raw
	}
	if err := readDurations(get, []durationSetting{
		{"ACCESS_TOKEN_TTL", 15 * time.Minute, &cfg.AccessTTL},
		{"REFRESH_TOKEN_TTL", 30 * 24 * time.Hour, &cfg.RefreshTTL},
		{"AUTH_REQUEST_TIMEOUT", 8 * time.Second, &cfg.RequestTimeout},
		{"AUTH_RATE_WINDOW", time.Minute, &cfg.RateWindow},
	}); err != nil {
		return cfg, err
	}
	if cfg.AccessTTL < time.Second || cfg.AccessTTL > 15*time.Minute || cfg.AccessTTL%time.Second != 0 {
		return cfg, errors.New("ACCESS_TOKEN_TTL must be whole seconds between 1s and 15m")
	}
	if cfg.RefreshTTL < cfg.AccessTTL || cfg.RefreshTTL > 30*24*time.Hour || cfg.RefreshTTL%time.Second != 0 {
		return cfg, errors.New("REFRESH_TOKEN_TTL must be whole seconds between ACCESS_TOKEN_TTL and 720h")
	}
	if cfg.RequestTimeout >= writeTimeout {
		return cfg, errors.New("AUTH_REQUEST_TIMEOUT must be shorter than HTTP_WRITE_TIMEOUT")
	}
	if cfg.RateWindow < time.Second || cfg.RateWindow > time.Hour {
		return cfg, errors.New("AUTH_RATE_WINDOW must be between 1s and 1h")
	}
	for _, item := range []struct {
		name       string
		dest       *int
		value, max int
	}{
		{"AUTH_RATE_IP_LIMIT", &cfg.IPLimit, 60, 10000},
		{"AUTH_RATE_ACCOUNT_LIMIT", &cfg.AccountLimit, 10, 1000},
		{"AUTH_RATE_MAX_KEYS", &cfg.RateKeys, 8192, 100000},
		{"AUTH_PASSWORD_WORKERS", &cfg.PasswordWorkers, 2, 8},
	} {
		*item.dest = item.value
		if raw := get(item.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > item.max {
				return cfg, fmt.Errorf("%s must be an integer between 1 and %d", item.name, item.max)
			}
			*item.dest = value
		}
	}
	return cfg, nil
}

func isLoopback(host string) bool {
	addr, err := netip.ParseAddr(host)
	return host == "localhost" || err == nil && addr.IsLoopback()
}
