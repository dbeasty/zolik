package ssh

import (
	"context"
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
	if err := ensureHostKey(cfg.HostKeyPath); err != nil {
		return nil, err
	}

	opts := []ssh.Option{
		wish.WithAddress(cfg.Addr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithMiddleware(
			bubblessh.Middleware(teaProgram(deps)),
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
		sess, err := deps.Auth.Login(ctx, user, password)
		if err != nil {
			return false
		}
		ctx.SetValue("zolik_session", sess)
		return true
	}))

	s, err := wish.NewServer(opts...)
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
	if v := s.Context().Value("zolik_session"); v != nil {
		if sess, ok := v.(Session); ok {
			return ui.PlayerSession{
				AccessToken:  sess.AccessToken,
				RefreshToken: sess.RefreshToken,
				UserID:       sess.UserID,
				Username:     sess.Username,
				IsGuest:      sess.IsGuest,
			}
		}
	}

	user := s.User()
	name := user
	if user == "guest" || user == "" {
		name = "Guest"
		if user == "" {
			user = "guest"
		}
	}
	ctx := s.Context()
	tok, err := deps.Auth.Guest(ctx, name)
	if err != nil {
		return ui.PlayerSession{Username: name, IsGuest: true}
	}
	return ui.PlayerSession{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		UserID:       tok.UserID,
		Username:     tok.Username,
		IsGuest:      tok.IsGuest,
	}
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
