package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

// healthcheck is what `server healthcheck` does: ask this container's own
// listener for /healthz and exit 0 only if it answers 200. The image is
// distroless — no shell, no curl, no wget — so the compose healthcheck has
// nothing to run but the server binary itself.
func healthcheck(port string) int {
	if port == "" {
		port = "8090"
	}
	c := http.Client{Timeout: 3 * time.Second}
	res, err := c.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: /healthz answered", res.Status)
		return 1
	}
	return 0
}
