package admission

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"zolik/server/internal/module"
)

// WriteBusy refuses an arrival there is no room for.
//
// 503 with Retry-After is the honest status: the caller did nothing wrong, the
// server is simply full, and the condition is temporary. The body carries
// SERVER_BUSY from the same vocabulary every other refusal uses, so a client
// renders it out of its locale bundle rather than showing a bare status code —
// and, because this is written before the WebSocket upgrade, a refused client
// reads it as an ordinary HTTP response instead of a socket that opens and
// then dies unexplained.
//
// Constructed as a module.Error so the key scanner sees the code — a bare map
// literal here silently drops SERVER_BUSY out of serverKeys.json.
func WriteBusy(w http.ResponseWriter, err error) {
	retry := 5 * time.Second
	var rej *Rejection
	if errors.As(err, &rej) && rej.RetryAfter > 0 {
		retry = rej.RetryAfter
	}
	busy := module.Error{Code: "SERVER_BUSY"}
	w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": busy.Code, "message": busy.Error()})
}
