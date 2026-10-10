package app

import "net/http"

// MaxRequestBody is the most any request body may carry. Every JSON route
// decodes its body straight off the connection, and without a bound a caller
// who has not signed in (the guest route, register, login) can make the server
// read and hold as much as they care to send. The largest honest body is a
// saved offline match, well under this.
const MaxRequestBody = 8 << 20

// LimitRequestBody caps what the handlers below it can read. A body past the
// cap fails the read in whichever handler asked for it, which every handler
// already answers as a bad request; routes that set a tighter cap of their own
// (the scorepad's 16 KiB) keep it.
func LimitRequestBody(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && r.Body != http.NoBody {
				r.Body = http.MaxBytesReader(w, r.Body, max)
			}
			next.ServeHTTP(w, r)
		})
	}
}
