package app

import (
	"os"
	"strings"
)

type Config struct {
	Env  string
	Port string

	MongoURI string
	MongoDB  string

	JWTAccessSecret  string
	JWTRefreshSecret string

	RedisURL   string
	InstanceID string

	SSHEnabled      bool
	SSHPort         string
	SSHHostKeyPath  string
	SSHAllowAllKeys bool
}

func LoadConfig() Config {
	env := os.Getenv("APP_ENV")
	local := env == "" || env == "local"
	return Config{
		Env:  env,
		Port: envOr("PORT", "8090"),

		MongoURI: envOr("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  envOr("MONGO_DB", "zolik"),

		JWTAccessSecret:  envOr("JWT_ACCESS_SECRET", "dev_access_secret_change_me"),
		JWTRefreshSecret: envOr("JWT_REFRESH_SECRET", "dev_refresh_secret_change_me"),

		RedisURL:   envOr("REDIS_URL", ""),
		InstanceID: envOr("INSTANCE_ID", ""),

		SSHEnabled:      envBool("SSH_ENABLED", local),
		SSHPort:         envOr("SSH_PORT", "2222"),
		SSHHostKeyPath:  envOr("SSH_HOST_KEY_PATH", ".ssh/zolik_host_key"),
		SSHAllowAllKeys: envBool("SSH_ALLOW_ALL_KEYS", local),
	}
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

