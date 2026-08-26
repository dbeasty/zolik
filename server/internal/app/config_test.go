package app

import (
	"context"
	"os"
	"testing"
	"time"
)

// unset removes a variable for the duration of one test. t.Setenv is called
// first purely for its cleanup: it records the original value and restores it
// when the test ends, which os.Unsetenv on its own would not.
func unset(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetting %s: %v", key, err)
	}
}

// REDIS_URL is resolved by whether it is *set*, not by whether it is empty —
// which is what keeps "set it to nothing" working as a way to say "no Redis".
// Collapsing those two cases would take that opt-out away.
func TestRedisConfigResolution(t *testing.T) {
	cases := []struct {
		name         string
		set          bool
		value        string
		local        bool
		wantURL      string
		wantOptional bool
	}{
		{
			name:  "unset locally falls back to the local default, but does not insist",
			local: true, wantURL: localRedisURL, wantOptional: true,
		},
		{
			name: "unset outside local means no Redis",
			// Guessing localhost in a deployment would be a guess about
			// someone else's network.
			local: false, wantURL: "", wantOptional: false,
		},
		{
			name: "set and empty is the explicit opt-out",
			set:  true, value: "", local: true, wantURL: "", wantOptional: false,
		},
		{
			name: "a configured URL is required, not optional",
			set:  true, value: "redis://elsewhere:6379/2", local: true,
			wantURL: "redis://elsewhere:6379/2", wantOptional: false,
		},
		{
			name: "a configured URL outside local is required too",
			set:  true, value: "redis://prod:6379/0", local: false,
			wantURL: "redis://prod:6379/0", wantOptional: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("REDIS_URL", tc.value)
			} else {
				unset(t, "REDIS_URL")
			}

			url, optional := redisConfig(tc.local)
			if url != tc.wantURL {
				t.Errorf("url = %q, want %q", url, tc.wantURL)
			}
			if optional != tc.wantOptional {
				t.Errorf("optional = %v, want %v", optional, tc.wantOptional)
			}
		})
	}
}

// deadRedis is a URL nothing answers on. Port 1 is reserved and unbindable by
// ordinary processes, so this fails to connect rather than finding something
// unrelated listening.
const deadRedis = "redis://127.0.0.1:1/0"

// The whole point of the optional default is that a machine with no Redis
// still starts. The other half matters just as much: a Redis someone
// *configured* must not be silently downgraded to local-only, because a
// deployment running as a lone instance while believing it is clustered is a
// far worse failure than one that refuses to boot.
func TestUnreachableRedisFallsBackOnlyWhenOptional(t *testing.T) {
	cases := []struct {
		name     string
		cfg      Config
		wantURL  string
		wantKept bool
	}{
		{
			name: "an unreachable default is dropped",
			cfg:  Config{RedisURL: deadRedis, RedisOptional: true},
		},
		{
			name:     "an unreachable configured URL is kept, so startup fails on it",
			cfg:      Config{RedisURL: deadRedis, RedisOptional: false},
			wantKept: true,
		},
		{
			name: "no URL stays no URL",
			cfg:  Config{RedisURL: "", RedisOptional: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			want := ""
			if tc.wantKept {
				want = tc.cfg.RedisURL
			}
			if got := resolveRedisURL(ctx, tc.cfg); got != want {
				t.Errorf("resolveRedisURL = %q, want %q", got, want)
			}
		})
	}
}

// The SSH terminal client binds a second port, so it stays off until asked
// for — a developer running two servers should not get a bind failure from a
// feature they are not using.
func TestSSHIsOffUnlessAskedFor(t *testing.T) {
	unset(t, "SSH_ENABLED")
	if LoadConfig().SSHEnabled {
		t.Error("SSH is enabled by default")
	}

	t.Setenv("SSH_ENABLED", "true")
	if !LoadConfig().SSHEnabled {
		t.Error("SSH_ENABLED=true did not enable it")
	}
}
