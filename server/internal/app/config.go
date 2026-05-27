package app

import "os"

type Config struct {
	Env string
	Port string

	MongoURI string
	MongoDB  string

	JWTAccessSecret  string
	JWTRefreshSecret string

	RedisURL   string
	InstanceID string
}

func LoadConfig() Config {
	return Config{
		Env: os.Getenv("APP_ENV"),
		Port: envOr("PORT", "8090"),

		MongoURI: envOr("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  envOr("MONGO_DB", "zolik"),

		JWTAccessSecret:  envOr("JWT_ACCESS_SECRET", "dev_access_secret_change_me"),
		JWTRefreshSecret: envOr("JWT_REFRESH_SECRET", "dev_refresh_secret_change_me"),

		RedisURL:   envOr("REDIS_URL", ""),
		InstanceID: envOr("INSTANCE_ID", ""),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

