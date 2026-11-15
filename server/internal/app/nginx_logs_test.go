package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The sockets carry the access token as ?token= (a browser cannot set a
// header on a WebSocket it opens), and nginx's default "combined" access log
// writes the whole request line. Every public vhost therefore logs through
// zolik_noquery, and that format must never name the query.
func TestNginxNeverLogsQueryStrings(t *testing.T) {
	dir := filepath.FromSlash("../../../deploy/nginx")
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}

	format := read("zolik-log-format.conf")
	if !strings.Contains(format, "log_format zolik_noquery") {
		t.Fatal("zolik-log-format.conf does not declare zolik_noquery")
	}
	for _, v := range []string{"$request ", "$request\"", "$request'", "$request_uri", "$args", "$query_string", "$arg_", "$is_args"} {
		if strings.Contains(format, v) {
			t.Errorf("zolik_noquery logs %s, which carries the query string", strings.Trim(v, " \"'"))
		}
	}

	const logs = "access_log /var/log/nginx/access.log zolik_noquery;"
	if !strings.Contains(read("zolik-proxy.conf"), logs) {
		t.Error("zolik-proxy.conf does not log through zolik_noquery")
	}
	// Every server block that does not include the shared snippet states it
	// itself: the plain-HTTP and www redirect vhosts see the same URLs.
	serverBlock := regexp.MustCompile(`(?s)server \{.*?\n\}`)
	for _, name := range []string{"jokerless.conf", "jokerless-http.conf", "play-limidus.conf"} {
		for _, block := range serverBlock.FindAllString(read(name), -1) {
			if !strings.Contains(block, "zolik-proxy.conf") && !strings.Contains(block, logs) {
				t.Errorf("a server block in %s logs with nginx's default format:\n%s", name, block)
			}
			if strings.Contains(block, "access_log") && !strings.Contains(block, "zolik_noquery") {
				t.Errorf("a server block in %s logs with another format", name)
			}
		}
	}

	// Installed where an http-level directive goes, before nginx -t runs on
	// the snippet that names it.
	deploy, err := os.ReadFile(filepath.FromSlash("../../../scripts/deploy.sh"))
	if err != nil {
		t.Fatal(err)
	}
	d := string(deploy)
	install := strings.Index(d, "/etc/nginx/conf.d/zolik-log-format.conf")
	if install < 0 {
		t.Fatal("deploy.sh does not install zolik-log-format.conf into conf.d")
	}
	if test := strings.Index(d[install:], "nginx -t"); test < 0 {
		t.Fatal("deploy.sh installs the format after the last nginx -t")
	}
}
