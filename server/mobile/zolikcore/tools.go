//go:build tools

package zolikcore

// gomobile bind generates code that imports this package, so it has to be in
// the module graph, but nothing in the server imports it. Without this line
// `go mod tidy` removes it and the next scripts/build-mobile-core.sh fails.
import _ "golang.org/x/mobile/bind"
