package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

// realisticGoSource is a representative Go file with multiple symbols.
const realisticGoSource = `package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// Config holds server configuration.
type Config struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Logger       *slog.Logger
}

// Server is the main HTTP server.
type Server struct {
	mu       sync.RWMutex
	config   Config
	router   *http.ServeMux
	handlers map[string]http.Handler
	started  bool
}

// Handler defines a route handler interface.
type Handler interface {
	Handle(ctx context.Context, req *http.Request) (any, error)
	Pattern() string
}

// HealthStatus represents the health check response.
type HealthStatus struct {
	Status    string    ` + "`json:\"status\"`" + `
	Uptime    float64   ` + "`json:\"uptime\"`" + `
	Timestamp time.Time ` + "`json:\"timestamp\"`" + `
}

const (
	DefaultPort         = 8080
	DefaultReadTimeout  = 30 * time.Second
	DefaultWriteTimeout = 30 * time.Second
	MaxHeaderBytes      = 1 << 20
)

var defaultLogger = slog.Default()

// New creates a new Server with the given config.
func New(cfg Config) *Server {
	if cfg.Port == 0 {
		cfg.Port = DefaultPort
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = DefaultReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = DefaultWriteTimeout
	}
	if cfg.Logger == nil {
		cfg.Logger = defaultLogger
	}

	return &Server{
		config:   cfg,
		router:   http.NewServeMux(),
		handlers: make(map[string]http.Handler),
	}
}

// Start begins listening on the configured address.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)
	s.config.Logger.Info("starting server", "addr", addr)

	srv := &http.Server{
		Addr:           addr,
		Handler:        s.router,
		ReadTimeout:    s.config.ReadTimeout,
		WriteTimeout:   s.config.WriteTimeout,
		MaxHeaderBytes: MaxHeaderBytes,
	}

	return srv.ListenAndServe()
}

// Register adds a handler for the given pattern.
func (s *Server) Register(pattern string, h http.Handler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.handlers[pattern] = h
	s.router.Handle(pattern, h)
}

// Health returns the current health status.
func (s *Server) Health() HealthStatus {
	return HealthStatus{
		Status:    "ok",
		Timestamp: time.Now(),
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.started = false
	s.config.Logger.Info("server shutdown complete")

	return nil
}
`

// realisticPythonSource is a representative Python file with multiple symbols.
const realisticPythonSource = `"""Authentication module for the API server."""

import hashlib
import hmac
import os
import time
from dataclasses import dataclass, field
from typing import Optional

DEFAULT_TOKEN_TTL = 3600
MAX_FAILED_ATTEMPTS = 5
LOCKOUT_DURATION = 900

@dataclass
class TokenPayload:
    """JWT-like token payload."""
    user_id: str
    email: str
    roles: list[str] = field(default_factory=list)
    issued_at: float = field(default_factory=time.time)
    expires_at: float = 0.0

@dataclass
class AuthResult:
    """Result of an authentication attempt."""
    success: bool
    token: Optional[str] = None
    error: Optional[str] = None
    user_id: Optional[str] = None

class PasswordHasher:
    """Handles password hashing and verification."""

    SALT_LENGTH = 32
    ITERATIONS = 100_000

    def __init__(self, algorithm: str = "sha256"):
        self.algorithm = algorithm

    def hash_password(self, password: str) -> str:
        salt = os.urandom(self.SALT_LENGTH)
        key = hashlib.pbkdf2_hmac(
            self.algorithm, password.encode(), salt, self.ITERATIONS
        )
        return salt.hex() + ":" + key.hex()

    def verify_password(self, password: str, stored: str) -> bool:
        parts = stored.split(":")
        if len(parts) != 2:
            return False
        salt = bytes.fromhex(parts[0])
        expected = bytes.fromhex(parts[1])
        key = hashlib.pbkdf2_hmac(
            self.algorithm, password.encode(), salt, self.ITERATIONS
        )
        return hmac.compare_digest(key, expected)

class TokenManager:
    """Manages authentication tokens."""

    def __init__(self, secret: str, ttl: int = DEFAULT_TOKEN_TTL):
        self.secret = secret
        self.ttl = ttl
        self._revoked: set[str] = set()

    def create_token(self, payload: TokenPayload) -> str:
        payload.expires_at = time.time() + self.ttl
        data = f"{payload.user_id}:{payload.email}:{payload.expires_at}"
        signature = hmac.new(
            self.secret.encode(), data.encode(), hashlib.sha256
        ).hexdigest()
        return f"{data}:{signature}"

    def validate_token(self, token: str) -> Optional[TokenPayload]:
        if token in self._revoked:
            return None
        parts = token.rsplit(":", 1)
        if len(parts) != 2:
            return None
        data, signature = parts
        expected = hmac.new(
            self.secret.encode(), data.encode(), hashlib.sha256
        ).hexdigest()
        if not hmac.compare_digest(signature, expected):
            return None
        fields = data.split(":")
        if len(fields) < 3:
            return None
        expires = float(fields[2])
        if time.time() > expires:
            return None
        return TokenPayload(user_id=fields[0], email=fields[1], expires_at=expires)

    def revoke_token(self, token: str) -> None:
        self._revoked.add(token)

def authenticate(username: str, password: str, hasher: PasswordHasher) -> AuthResult:
    """Authenticate a user with username and password."""
    if not username or not password:
        return AuthResult(success=False, error="missing credentials")
    return AuthResult(success=True, user_id=username)
`

// writeTempFile writes content to a temp file with the given extension and returns the path.
func writeTempFile(b *testing.B, ext, content string) string {
	b.Helper()

	dir := b.TempDir()
	path := filepath.Join(dir, "bench"+ext)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("write temp file: %v", err)
	}

	return path
}

// BenchmarkParseGoFile benchmarks parsing a realistic Go source file.
func BenchmarkParseGoFile(b *testing.B) {
	path := writeTempFile(b, ".go", realisticGoSource)

	b.ResetTimer()

	for range b.N {
		_, _ = ParseGoFile(path)
	}
}

// BenchmarkParsePythonFile benchmarks parsing a realistic Python source file.
func BenchmarkParsePythonFile(b *testing.B) {
	path := writeTempFile(b, ".py", realisticPythonSource)

	b.ResetTimer()

	for range b.N {
		_, _ = ParsePythonFile(path)
	}
}
