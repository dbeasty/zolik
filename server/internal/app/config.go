package app

import (
	"os"
	"strconv"
	"strings"

	"zolik/server/internal/auth"
	"zolik/server/internal/db"
	"zolik/server/internal/identity"
)

type Config struct {
	Env  string
	Port string

	// DBEngine selects the storage backend: db.EngineMongo (the default) or
	// db.EngineKDB, which runs the embedded KDB engine in-process — one
	// binary, no database server, no Redis. See LoadConfig for the flags.
	DBEngine string

	MongoURI string
	MongoDB  string

	// KDBPath is the directory the embedded KDB engine persists to when
	// DBEngine is db.EngineKDB. Empty falls back to in-memory storage, which
	// loses everything on restart — acceptable only in tests.
	KDBPath string

	JWTAccessSecret  string
	JWTRefreshSecret string

	RedisURL   string
	InstanceID string

	SSHEnabled      bool
	SSHPort         string
	SSHHostKeyPath  string
	SSHAllowAllKeys bool

	// PublicBaseURL is how the outside world reaches this server. It is what
	// the OAuth redirect URI is built from, and that URI has to match what is
	// registered with each provider byte-for-byte — so this is configuration
	// rather than something derived from the incoming request, which an
	// attacker could otherwise influence via the Host header.
	PublicBaseURL string
	// AllowedReturnURLs are the prefixes a browser sign-in may hand control
	// back to. It is an open-redirect allow-list guarding a redirect that
	// carries a code exchangeable for a session — see Handlers.resolveReturnTo.
	AllowedReturnURLs []string

	// Identity is the external sign-in configuration. Providers with no client
	// id are simply not offered.
	Identity identity.Config

	// SMTP delivers passwordless sign-in codes. Without a host, local
	// development logs the codes instead of mailing them; any other
	// environment refuses to start rather than silently swallowing them.
	SMTP auth.SMTPConfig

	// TestEndpointsEnabled gates /games/{id}/debug-state, which writes a
	// game's phase/hands/melds/turn directly into Mongo, bypassing rules
	// validation — lets e2e tests jump straight into a specific mid-round
	// UI state instead of playing a full deal turn-by-turn. Defaults on for
	// local dev (same as SSHAllowAllKeys) but must be explicitly opted into
	// anywhere APP_ENV is set.
	TestEndpointsEnabled bool

	// AdmissionMaxConnections caps concurrently held sockets. Zero (the
	// default) derives a ceiling from the process's memory limit; -1 turns
	// the count ceiling off entirely. On hosts with no readable limit — dev
	// machines — zero also means no ceiling, so nothing changes there.
	AdmissionMaxConnections int
	// AdmissionWaitingRoomRatio is the fraction of the ceiling past which
	// waiting-room sockets (and new matches) are refused while gameplay
	// sockets still get the reserved tail.
	AdmissionWaitingRoomRatio float64
	// AdmissionMemoryWatermark is the fraction of the memory limit at which
	// new connections stop being admitted. Zero disables the memory gate.
	AdmissionMemoryWatermark float64
	// AdmissionCPUWatermark is the CPU stall fraction (PSI some avg10, 0..1)
	// past which the server stops growing: no new waiting-room sockets, no
	// new matches. Zero disables the CPU gate.
	AdmissionCPUWatermark float64
}

// LoadConfig reads the environment.
//
// Every identity provider is read the same way, from the same three variable
// names with a different prefix, so enabling Apple or Microsoft later is a
// matter of setting variables rather than editing this function.
func LoadConfig() Config {
	env := os.Getenv("APP_ENV")
	local := env == "" || env == "local"
	publicBaseURL := envOr("PUBLIC_BASE_URL", "http://localhost:"+envOr("PORT", "8090"))
	return Config{
		Env:  env,
		Port: envOr("PORT", "8090"),

		DBEngine: dbEngine(),

		MongoURI: envOr("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:  envOr("MONGO_DB", "zolik"),

		KDBPath: envOr("KDB_PATH", "data/kdb"),

		JWTAccessSecret:  envOr("JWT_ACCESS_SECRET", "dev_access_secret_change_me"),
		JWTRefreshSecret: envOr("JWT_REFRESH_SECRET", "dev_refresh_secret_change_me"),

		RedisURL:   envOr("REDIS_URL", ""),
		InstanceID: envOr("INSTANCE_ID", ""),

		SSHEnabled:      envBool("SSH_ENABLED", local),
		SSHPort:         envOr("SSH_PORT", "2222"),
		SSHHostKeyPath:  envOr("SSH_HOST_KEY_PATH", ".ssh/zolik_host_key"),
		SSHAllowAllKeys: envBool("SSH_ALLOW_ALL_KEYS", local),

		PublicBaseURL: publicBaseURL,
		// The app's deep-link scheme ("clientreactnative://", per
		// client-react-native/app.json) is allowed by default so the mobile
		// client works out of the box; the server's own origin covers the web
		// build. Anything else has to be declared.
		AllowedReturnURLs: envList("AUTH_ALLOWED_RETURN_URLS", []string{"clientreactnative://", publicBaseURL}),

		Identity: identity.Config{
			Google: providerConfig("GOOGLE"),
			Apple: identity.ProviderConfig{
				ClientID:       os.Getenv("OAUTH_APPLE_CLIENT_ID"),
				ExtraAudiences: envList("OAUTH_APPLE_EXTRA_AUDIENCES", nil),
				TeamID:         os.Getenv("OAUTH_APPLE_TEAM_ID"),
				KeyID:          os.Getenv("OAUTH_APPLE_KEY_ID"),
				// Either the PEM itself or a path to the .p8 file.
				PrivateKey: os.Getenv("OAUTH_APPLE_PRIVATE_KEY"),
			},
			Microsoft: identity.ProviderConfig{
				ClientID:       os.Getenv("OAUTH_MICROSOFT_CLIENT_ID"),
				ClientSecret:   os.Getenv("OAUTH_MICROSOFT_CLIENT_SECRET"),
				ExtraAudiences: envList("OAUTH_MICROSOFT_EXTRA_AUDIENCES", nil),
				Tenant:         envOr("OAUTH_MICROSOFT_TENANT", "common"),
			},
		},

		SMTP: auth.SMTPConfig{
			Host:     os.Getenv("SMTP_HOST"),
			Port:     envOr("SMTP_PORT", "587"),
			Username: os.Getenv("SMTP_USERNAME"),
			Password: os.Getenv("SMTP_PASSWORD"),
			From:     os.Getenv("SMTP_FROM"),
			FromName: envOr("SMTP_FROM_NAME", "Žolíky"),
		},

		TestEndpointsEnabled: envBool("ENABLE_TEST_ENDPOINTS", local),

		// Watermark defaults follow the admission package's measurements: 0.85
		// leaves slack for the lag between admitting a connection and its
		// memory landing; 0.25 CPU stall is well past comfortable and still
		// short of seized. Both gates disarm themselves on hosts where the
		// reading does not exist, so local development is unaffected.
		AdmissionMaxConnections:   envInt("ADMISSION_MAX_CONNECTIONS", 0),
		AdmissionWaitingRoomRatio: envFloat("ADMISSION_WAITING_ROOM_RATIO", 0.8),
		AdmissionMemoryWatermark:  envFloat("ADMISSION_MEMORY_WATERMARK", 0.85),
		AdmissionCPUWatermark:     envFloat("ADMISSION_CPU_WATERMARK", 0.25),
	}
}

// dbEngine reads the storage-backend feature flag. FEATURE_FLAG_DB_ENGINE
// names the engine outright ("kdb" or "mongo"); FEATURE_FLAG_KDB=true is the
// boolean spelling of the same choice. Either works in a deployment's
// environment; an unrecognised value falls back to Mongo, the engine every
// existing deployment is already running on.
func dbEngine() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("FEATURE_FLAG_DB_ENGINE"))) {
	case "kdb":
		return db.EngineKDB
	case "mongo", "mongodb":
		return db.EngineMongo
	}
	if envBool("FEATURE_FLAG_KDB", false) {
		return db.EngineKDB
	}
	return db.EngineMongo
}

// providerConfig reads the standard three variables for an OAuth provider.
func providerConfig(prefix string) identity.ProviderConfig {
	return identity.ProviderConfig{
		ClientID:     os.Getenv("OAUTH_" + prefix + "_CLIENT_ID"),
		ClientSecret: os.Getenv("OAUTH_" + prefix + "_CLIENT_SECRET"),
		// Native SDKs sign in under their own platform client id, all of which
		// must be accepted alongside the server's.
		ExtraAudiences: envList("OAUTH_"+prefix+"_EXTRA_AUDIENCES", nil),
	}
}

// envList reads a comma-separated variable, dropping blanks.
func envList(key string, fallback []string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
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

// envInt and envFloat follow envBool's manner: an unset, empty, or
// unparseable value is the fallback, never an error — configuration here is
// lenient by convention, and the storage-level flags are the strict ones.
func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
