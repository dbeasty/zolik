package app

import "testing"

// An unset APP_ENV used to mean local: test endpoints on, any SSH key
// admitted, the published dev signing secret accepted. A deployment whose env
// file lost the line came up wide open. Now local has to be asked for.
func TestAnUnsetAppEnvIsProduction(t *testing.T) {
	for _, unset := range []string{"", "  "} {
		t.Setenv("APP_ENV", unset)
		for _, k := range []string{"ENABLE_TEST_ENDPOINTS", "ENABLE_DEBUG_ENDPOINTS", "SSH_ENABLED", "SSH_ALLOW_ALL_KEYS"} {
			t.Setenv(k, "")
		}
		cfg := LoadConfig()
		if cfg.IsLocal() {
			t.Fatalf("APP_ENV=%q is local", unset)
		}
		if cfg.Env != EnvProduction {
			t.Errorf("APP_ENV=%q: Env = %q, want %q", unset, cfg.Env, EnvProduction)
		}
		if cfg.TestEndpointsEnabled || cfg.DebugEndpointsEnabled || cfg.SSHEnabled || cfg.SSHAllowAllKeys {
			t.Errorf("APP_ENV=%q opened a hatch: test=%v debug=%v ssh=%v anykey=%v", unset,
				cfg.TestEndpointsEnabled, cfg.DebugEndpointsEnabled, cfg.SSHEnabled, cfg.SSHAllowAllKeys)
		}
	}
}

func TestLocalAppEnvStillOpensTheDevelopmentDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "local")
	for _, k := range []string{"ENABLE_TEST_ENDPOINTS", "SSH_ALLOW_ALL_KEYS"} {
		t.Setenv(k, "")
	}
	cfg := LoadConfig()
	if !cfg.IsLocal() || !cfg.TestEndpointsEnabled || !cfg.SSHAllowAllKeys {
		t.Fatalf("APP_ENV=local: local=%v test=%v anykey=%v", cfg.IsLocal(), cfg.TestEndpointsEnabled, cfg.SSHAllowAllKeys)
	}
}
