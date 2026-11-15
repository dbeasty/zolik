package ssh

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bubblessh "github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"

	tea "github.com/charmbracelet/bubbletea"

	"zolik/client-tui/ui"
)

type Config struct {
	Addr         string
	HostKeyPath  string
	AllowAllKeys bool
}

type Deps struct {
	ServerURL string
	Auth      Authenticator
	// Build is the server's own build identity, threaded straight through to
	// every SSH session's ui.Root — see ui.Build's doc comment for why this
	// is passed as data rather than linked into client-tui as a second copy.
	Build ui.Build
}

type Server struct {
	srv *ssh.Server
}

func Start(ctx context.Context, cfg Config, deps Deps) (*Server, error) {
	s, err := newServer(cfg, deps, newGate(nil))
	if err != nil {
		return nil, err
	}

	srv := &Server{srv: s}
	go func() {
		<-ctx.Done()
		_ = s.Shutdown(ctx)
	}()

	go func() {
		log.Printf("ssh tui listening on %s", cfg.Addr)
		if err := s.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			log.Printf("ssh server: %v", err)
		}
	}()

	return srv, nil
}

// sessionKey is where a connection's Session waits in its ssh.Context, from
// the password check or the guest middleware until the UI reads it.
const sessionKey = "zolik_session"

func newServer(cfg Config, deps Deps, g *gate) (*ssh.Server, error) {
	if err := ensureHostKey(cfg.HostKeyPath); err != nil {
		return nil, err
	}

	opts := []ssh.Option{
		wish.WithAddress(cfg.Addr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		// The last middleware runs first: log, then settle who this is, then
		// hand the terminal to the UI.
		wish.WithMiddleware(
			bubblessh.Middleware(teaProgram(deps)),
			guestMiddleware(g, deps),
			logging.Middleware(),
		),
	}

	if cfg.AllowAllKeys {
		opts = append(opts, wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			return true
		}))
	}

	opts = append(opts, wish.WithPasswordAuth(func(ctx ssh.Context, password string) bool {
		user := ctx.User()
		if user == "guest" {
			return true
		}
		sess, ok := g.login(ctx, deps.Auth, user, password, ctx.RemoteAddr())
		if !ok {
			return false
		}
		ctx.SetValue(sessionKey, sess)
		return true
	}))

	return wish.NewServer(opts...)
}

// guestMiddleware gives a connection that did not sign in its guest session
// before the UI starts, so that one that may not have one can be turned away
// with a reason rather than shown a table it cannot sit at.
func guestMiddleware(g *gate, deps Deps) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			ctx := s.Context()
			if _, ok := ctx.Value(sessionKey).(Session); !ok {
				sess, err := g.guest(ctx, deps.Auth, s.User(), s.PublicKey(), s.RemoteAddr())
				switch {
				case errors.Is(err, errTooManyGuests):
					wish.Fatalln(s, "Too many new guest sessions from your address. Try again later, or sign in.")
					return
				case err == nil:
					ctx.SetValue(sessionKey, sess)
				}
				// Any other error leaves the connection without a session, and
				// the UI shows a player who is not signed in, as before.
			}
			next(s)
		}
	}
}

func teaProgram(deps Deps) bubblessh.Handler {
	return func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
		sess := sessionForSSH(s, deps)
		m := ui.NewRoot(s, deps.ServerURL, sess, deps.Build)
		opts := []tea.ProgramOption{
			tea.WithInput(s),
			tea.WithOutput(s),
		}
		renderer := bubblessh.MakeRenderer(s)
		if renderer != nil {
			m.SetRenderer(renderer)
		}
		return m, opts
	}
}

func sessionForSSH(s ssh.Session, deps Deps) ui.PlayerSession {
	if sess, ok := s.Context().Value(sessionKey).(Session); ok {
		return ui.PlayerSession{
			AccessToken:  sess.AccessToken,
			RefreshToken: sess.RefreshToken,
			UserID:       sess.UserID,
			Username:     sess.Username,
			IsGuest:      sess.IsGuest,
		}
	}
	// No session, so no invented name either: this is a placeholder for a
	// player who is not signed in to anything.
	name := s.User()
	if name == "guest" || name == "" {
		name = "Guest"
	}
	return ui.PlayerSession{Username: name, IsGuest: true}
}

func ensureHostKey(path string) error {
	if path == "" {
		return fmt.Errorf("empty host key path")
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

// AddrFromPort returns a listen address for the given port.
func AddrFromPort(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		port = "2222"
	}
	return net.JoinHostPort("0.0.0.0", port)
}
