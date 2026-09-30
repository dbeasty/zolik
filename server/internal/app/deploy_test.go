package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// The deployment is now two compose files stating one container shape: the
// development stack builds from source, the production stack runs a loaded
// image, and everything between those two facts is meant to be identical.
// Nothing enforces that by construction, so these tests do.
//
// This is the same job deploy/nginx's route list used to need doing for it:
// a value that has to be repeated in two places will eventually be changed in
// one, and the failure will not look like a configuration mistake.
const (
	prodCompose = "../../../deploy/compose/zolik.yml"
	devCompose  = "../../docker-compose.kdb.yml"
)

type composeFile struct {
	Name     string `yaml:"name"`
	Services map[string]struct {
		Image       string         `yaml:"image"`
		Build       any            `yaml:"build"`
		Environment map[string]any `yaml:"environment"`
		Ports       []string       `yaml:"ports"`
	} `yaml:"services"`
	Volumes map[string]struct {
		Name string `yaml:"name"`
	} `yaml:"volumes"`
}

func readCompose(t *testing.T, path string) composeFile {
	t.Helper()

	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var out composeFile
	if err := yaml.Unmarshal(raw, &out); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if _, ok := out.Services["app"]; !ok {
		t.Fatalf("%s has no `app` service", path)
	}
	return out
}

// TestProductionComposeShipsAnImage pins the thing that makes the host
// source-free. A `build:` key here would put the Go toolchain, the npm
// install and both source trees back on the production machine, which is the
// arrangement this whole change exists to end.
func TestProductionComposeShipsAnImage(t *testing.T) {
	prod := readCompose(t, prodCompose)
	app := prod.Services["app"]

	if app.Build != nil {
		t.Error("the production compose file has a `build:` key — the host has no source to build from")
	}
	if !strings.Contains(app.Image, "ZOLIK_RELEASE") {
		t.Errorf("production image is %q; it must name ${ZOLIK_RELEASE} so a deploy and a rollback "+
			"both say which build they mean", app.Image)
	}
	// `:?` rather than `:-`. A default would resolve an unset variable to some
	// tag that exists, which is how you deploy last week's build believing it
	// is this week's.
	if !strings.Contains(app.Image, ":?") {
		t.Errorf("production image is %q; ZOLIK_RELEASE must use `:?` so an unset variable "+
			"refuses to start instead of silently running whatever is loaded", app.Image)
	}
}

// TestProductionDataOutlivesTheDeploy is the guard on hazard 1 — the one that
// loses the database without producing an error.
//
// Compose derives a project name from the directory holding the compose file
// and prefixes volume names with it. Unpinned, moving or renaming that
// directory silently points the container at a new, empty volume: the
// container is healthy, /healthz says ok, the sign-up page works, and every
// account is gone. Both names are stated in the file so that neither depends
// on a path.
func TestProductionDataOutlivesTheDeploy(t *testing.T) {
	prod := readCompose(t, prodCompose)

	if prod.Name == "" {
		t.Error("the production compose file sets no top-level `name:` — the project name would " +
			"come from whichever directory it is run from, and so would the volume prefix")
	}

	vol, ok := prod.Volumes["kdb_data"]
	if !ok {
		t.Fatal("the production compose file declares no kdb_data volume — the database would " +
			"live in the container and vanish on the next deploy")
	}
	if vol.Name == "" {
		t.Error("kdb_data has no pinned `name:` — its real name would be derived from the project, " +
			"and moving this file would create a new empty volume rather than an error")
	}
}

// TestProductionClosesTheHatches checks the three switches that are dangerous
// on a public host, and checks them as literals.
//
// `${ENABLE_TEST_ENDPOINTS:-false}` reads as safe and is not: it hands the
// decision to whatever environment the deploy happens to run in. On a host
// where one of these is exported for any reason, the interpolated form opens
// it. The development file may interpolate — the end-to-end suite needs to
// open them deliberately — and production may not.
func TestProductionClosesTheHatches(t *testing.T) {
	env := readCompose(t, prodCompose).Services["app"].Environment

	for _, key := range []string{"SSH_ENABLED", "SSH_ALLOW_ALL_KEYS", "ENABLE_TEST_ENDPOINTS", "ENABLE_DEBUG_ENDPOINTS"} {
		value, ok := env[key]
		if !ok {
			t.Errorf("%s is unset in production. It is not off by omission: APP_ENV=local turns "+
				"all three on, and the env file is one edit away from saying local", key)
			continue
		}
		got := strings.TrimSpace(strings.ToLower(toString(value)))
		if strings.Contains(got, "${") {
			t.Errorf("%s is %q — an interpolated value lets the host's environment decide "+
				"whether it is open", key, got)
		} else if got != "false" {
			t.Errorf("%s = %q in production, want a literal false", key, got)
		}
	}
}

// TestComposeFilesAgreeOnTheContainer catches drift between the stack that is
// developed against and the stack that is deployed. A key added to one and
// forgotten in the other means local testing stops describing production, and
// nothing says so until something behaves differently on the host.
func TestComposeFilesAgreeOnTheContainer(t *testing.T) {
	prod := readCompose(t, prodCompose).Services["app"].Environment
	dev := readCompose(t, devCompose).Services["app"].Environment

	// Knobs that exist for the end-to-end suite and have no business being
	// pinned on a public host: the bot pause is cosmetic, and the suite sets
	// it near zero so its timeouts measure the code rather than the sleep.
	// Anything not listed here must appear in both files.
	devOnly := map[string]bool{
		"BOT_THINK_MIN_MS": true,
		"BOT_THINK_MAX_MS": true,
	}

	for key := range dev {
		if devOnly[key] {
			continue
		}
		if _, ok := prod[key]; !ok {
			t.Errorf("%s configures %s but %s does not. Either set it there too, or add it to "+
				"devOnly above with a reason", devCompose, key, prodCompose)
		}
	}

	// The storage engine and its path decide where the database is and
	// whether the volume mount reaches it. These two disagreeing is a fresh
	// empty database with no error, from a different direction than the
	// volume name.
	for _, key := range []string{"FEATURE_FLAG_DB_ENGINE", "KDB_PATH"} {
		if toString(dev[key]) != toString(prod[key]) {
			t.Errorf("%s = %q in development but %q in production", key, toString(dev[key]), toString(prod[key]))
		}
	}
}

// toString flattens the values YAML hands back: compose environment values are
// quoted strings ("false") or bare scalars (false), and both mean the same
// thing to Docker.
func toString(v any) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return value
	case bool:
		if value {
			return "true"
		}
		return "false"
	default:
		out, err := yaml.Marshal(value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
}

// TestAdminConsoleIsPublishedWhereItListens ties together the two halves of
// the console's isolation, which are stated in different files and were once
// both loopback — an arrangement that reads like defence in depth and is
// actually a wall. Docker forwards a published port to the container's bridge
// address; a console bound to the container's loopback is not listening
// there, so every tunnelled request is accepted and then dropped. The symptom
// is an empty reply while the startup log cheerfully reports the console up,
// which is about as far from a useful error as a running process gets.
//
// So: the published port must stay on the host's loopback, because that is
// the binding Docker enforces and the only thing keeping the console off the
// LAN — and ADMIN_BIND must not be a loopback address, because that one keeps
// it away from everybody.
func TestAdminConsoleIsPublishedWhereItListens(t *testing.T) {
	app := readCompose(t, prodCompose).Services["app"]

	var published string
	for _, p := range app.Ports {
		if strings.HasSuffix(p, ":8091") || strings.Contains(p, ":8091:") {
			published = p
			break
		}
	}
	if published == "" {
		// Nothing to keep honest: no admin port is published at all.
		return
	}

	if !strings.HasPrefix(published, "127.0.0.1:") {
		t.Errorf("the console is published as %q. Anything but a 127.0.0.1: prefix binds every "+
			"interface on the host, which puts the console on the LAN and, through the WAN NAT "+
			"that forwards to that box, potentially further", published)
	}

	bind := strings.TrimSpace(toString(app.Environment["ADMIN_BIND"]))
	if bind == "" {
		t.Fatalf("the console is published as %q but ADMIN_BIND is unset, so the server falls back "+
			"to its 127.0.0.1 default — inside the container, where Docker cannot reach it. The "+
			"console would start, log that it is up, and answer nobody", published)
	}
	if bind == "127.0.0.1" || bind == "localhost" || bind == "::1" {
		t.Errorf("ADMIN_BIND = %q binds the *container's* loopback, which the published port %q "+
			"cannot reach. Use 0.0.0.0 and let the 127.0.0.1: prefix above be the boundary",
			bind, published)
	}
}
