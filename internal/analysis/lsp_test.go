package analysis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// mockLSPClient implements LSPClient for testing without subprocess spawning.
type mockLSPClient struct {
	initialized bool
	rootDir     string
	symbols     map[string][]Symbol
	references  map[string][]Reference
	shutdownCalled bool
}

func newMockLSPClient() *mockLSPClient {
	return &mockLSPClient{
		symbols:    make(map[string][]Symbol),
		references: make(map[string][]Reference),
	}
}

func (m *mockLSPClient) Initialize(rootDir string) error {
	m.initialized = true
	m.rootDir = rootDir

	return nil
}

func (m *mockLSPClient) Symbols(path string) ([]Symbol, error) {
	if !m.initialized {
		return nil, fmt.Errorf("not initialized")
	}

	if syms, ok := m.symbols[path]; ok {
		return syms, nil
	}

	return nil, fmt.Errorf("no symbols for %s", path)
}

func (m *mockLSPClient) References(path string, line int) ([]Reference, error) {
	if !m.initialized {
		return nil, fmt.Errorf("not initialized")
	}

	key := fmt.Sprintf("%s:%d", path, line)
	if refs, ok := m.references[key]; ok {
		return refs, nil
	}

	return nil, nil
}

func (m *mockLSPClient) Shutdown() error {
	m.shutdownCalled = true
	m.initialized = false

	return nil
}

func TestMockLSPClientInterface(t *testing.T) {
	// Verify mockLSPClient satisfies LSPClient interface.
	var _ LSPClient = newMockLSPClient()
}

func TestMockLSPClientSymbols(t *testing.T) {
	client := newMockLSPClient()

	// Should fail before initialization.
	_, err := client.Symbols("test.go")
	if err == nil {
		t.Error("expected error before initialization")
	}

	if err := client.Initialize("/tmp/project"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	// Add mock symbols.
	client.symbols["test.go"] = []Symbol{
		{Name: "Server", Kind: KindStruct, File: "test.go", Line: 5, Exported: true},
		{Name: "New", Kind: KindFunction, File: "test.go", Line: 10, Exported: true},
	}

	syms, err := client.Symbols("test.go")
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}

	if len(syms) != 2 {
		t.Errorf("got %d symbols, want 2", len(syms))
	}

	if syms[0].Name != "Server" {
		t.Errorf("first symbol = %q, want %q", syms[0].Name, "Server")
	}
}

func TestMockLSPClientReferences(t *testing.T) {
	client := newMockLSPClient()

	if err := client.Initialize("/tmp/project"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	client.references["test.go:5"] = []Reference{
		{File: "other.go", Line: 20, Kind: "reference"},
	}

	refs, err := client.References("test.go", 5)
	if err != nil {
		t.Fatalf("References: %v", err)
	}

	if len(refs) != 1 {
		t.Errorf("got %d references, want 1", len(refs))
	}
}

func TestMockLSPClientShutdown(t *testing.T) {
	client := newMockLSPClient()

	if err := client.Initialize("/tmp/project"); err != nil {
		t.Fatalf("Initialize: %v", err)
	}

	if err := client.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if !client.shutdownCalled {
		t.Error("shutdown was not called")
	}

	// Symbols should fail after shutdown.
	_, err := client.Symbols("test.go")
	if err == nil {
		t.Error("expected error after shutdown")
	}
}

func TestNewLSPClientEmptyCommand(t *testing.T) {
	_, err := NewLSPClient(LSPConfig{})
	if err == nil {
		t.Error("expected error for empty server command")
	}
}

func TestNewLSPClientCreation(t *testing.T) {
	client, err := NewLSPClient(LSPConfig{
		ServerCommand: "gopls",
		Languages:     []Language{LangGo},
	})
	if err != nil {
		t.Fatalf("NewLSPClient: %v", err)
	}

	if client == nil {
		t.Error("expected non-nil client")
	}
}

func TestPathToURI(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/project/main.go", "file:///home/user/project/main.go"},
		{"C:/Users/test/project/main.go", "file:///C:/Users/test/project/main.go"},
		{"/tmp/test.py", "file:///tmp/test.py"},
	}

	for _, tt := range tests {
		got := pathToURI(tt.path)
		if got != tt.want {
			t.Errorf("pathToURI(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/project/main.go", "/home/user/project/main.go"},
		{"file:///C:/Users/test/main.go", "C:/Users/test/main.go"},
	}

	for _, tt := range tests {
		got := uriToPath(tt.uri)
		if got != tt.want {
			t.Errorf("uriToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestLSPSymbolKindMapping(t *testing.T) {
	tests := []struct {
		lspKind int
		want    SymbolKind
	}{
		{5, KindStruct},    // Class
		{6, KindMethod},    // Method
		{11, KindInterface}, // Interface
		{12, KindFunction},  // Function
		{13, KindVariable},  // Variable
		{14, KindConstant},  // Constant
		{23, KindStruct},    // Struct
		{99, KindVariable},  // Unknown -> Variable
	}

	for _, tt := range tests {
		got := lspSymbolKindToKind(tt.lspKind)
		if got != tt.want {
			t.Errorf("lspSymbolKindToKind(%d) = %q, want %q", tt.lspKind, got, tt.want)
		}
	}
}

func TestLSPConfig(t *testing.T) {
	cfg := LSPConfig{
		ServerCommand: "gopls",
		ServerArgs:    []string{"-remote=auto"},
		Languages:     []Language{LangGo},
	}

	if cfg.ServerCommand != "gopls" {
		t.Errorf("ServerCommand = %q, want %q", cfg.ServerCommand, "gopls")
	}

	if len(cfg.Languages) != 1 || cfg.Languages[0] != LangGo {
		t.Errorf("Languages = %v, want [go]", cfg.Languages)
	}
}

func TestWithLSPOption(t *testing.T) {
	configs := []LSPConfig{
		{
			ServerCommand: "gopls",
			Languages:     []Language{LangGo},
		},
		{
			ServerCommand: "pyright",
			Languages:     []Language{LangPython},
		},
	}

	a := NewAnalyzer("/tmp/project", WithLSP(configs))

	if _, ok := a.lspConfigs[LangGo]; !ok {
		t.Error("expected LSP config for Go")
	}

	if _, ok := a.lspConfigs[LangPython]; !ok {
		t.Error("expected LSP config for Python")
	}

	if _, ok := a.lspConfigs[LangRust]; ok {
		t.Error("unexpected LSP config for Rust")
	}
}

func TestAnalyzerWithLSPFallback(t *testing.T) {
	// Create a test Go file.
	dir := t.TempDir()
	goFile := filepath.Join(dir, "main.go")
	_ = os.WriteFile(goFile, []byte(`package main

func Hello() {}
`), 0o644)

	// Create analyzer with a broken LSP config (command doesn't exist).
	// It should fall back to regex parser.
	configs := []LSPConfig{
		{
			ServerCommand: "nonexistent-lsp-server-xxxxx",
			Languages:     []Language{LangGo},
		},
	}

	a := NewAnalyzer(dir, WithLSP(configs))

	fr, err := a.AnalyzeFile(goFile)
	if err != nil {
		t.Fatalf("AnalyzeFile: %v", err)
	}

	if fr.Package != "main" {
		t.Errorf("Package = %q, want %q", fr.Package, "main")
	}

	if len(fr.Symbols) == 0 {
		t.Error("expected symbols from fallback regex parser")
	}
}

func TestAnalyzerNoLSPByDefault(t *testing.T) {
	// Ensure THIMBLE_LSP_GOPLS is not set.
	t.Setenv("THIMBLE_LSP_GOPLS", "")

	a := NewAnalyzer("/tmp/project")

	if len(a.lspConfigs) != 0 {
		t.Errorf("expected no LSP configs by default, got %d", len(a.lspConfigs))
	}
}

func TestAnalyzerLSPEnvVar(t *testing.T) {
	t.Setenv("THIMBLE_LSP_GOPLS", "1")

	a := NewAnalyzer("/tmp/project")

	if _, ok := a.lspConfigs[LangGo]; !ok {
		t.Error("expected Go LSP config when THIMBLE_LSP_GOPLS=1")
	}
}

func TestLSPEnabledEnvVar(t *testing.T) {
	// Not set -> disabled.
	t.Setenv("THIMBLE_LSP_GOPLS", "")

	if lspEnabled(LangGo) {
		t.Error("expected Go LSP disabled when env not set")
	}

	// Set to 1 -> enabled.
	t.Setenv("THIMBLE_LSP_GOPLS", "1")

	if !lspEnabled(LangGo) {
		t.Error("expected Go LSP enabled when THIMBLE_LSP_GOPLS=1")
	}

	// Unsupported language -> always disabled.
	if lspEnabled(LangRust) {
		t.Error("expected Rust LSP disabled (not supported yet)")
	}
}

func TestDefaultLSPConfig(t *testing.T) {
	goCfg := defaultLSPConfig(LangGo)
	if goCfg == nil {
		t.Fatal("expected non-nil Go LSP config")
	}

	if goCfg.ServerCommand != "gopls" {
		t.Errorf("Go LSP command = %q, want %q", goCfg.ServerCommand, "gopls")
	}

	rustCfg := defaultLSPConfig(LangRust)
	if rustCfg != nil {
		t.Error("expected nil Rust LSP config (not supported yet)")
	}
}

func TestShutdownLSP(t *testing.T) {
	a := NewAnalyzer("/tmp/project")

	mock := newMockLSPClient()
	_ = mock.Initialize("/tmp/project")
	a.lspClients[LangGo] = mock

	a.ShutdownLSP()

	if !mock.shutdownCalled {
		t.Error("expected LSP client shutdown to be called")
	}

	if len(a.lspClients) != 0 {
		t.Errorf("expected empty lspClients after shutdown, got %d", len(a.lspClients))
	}
}

// TestJSONRPCProtocol tests the JSON-RPC message encoding/decoding by
// simulating an LSP server using an in-memory pipe.
func TestJSONRPCProtocol(t *testing.T) {
	// Create a pipe to simulate stdin/stdout of an LSP server.
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	// Restore SpawnFunc after test.
	origSpawn := SpawnFunc

	t.Cleanup(func() { SpawnFunc = origSpawn })

	// Find a free port to create a unique "command" that identifies our mock.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	mockCmd := fmt.Sprintf("mock-lsp-%d", port)

	SpawnFunc = func(name string, args ...string) *exec.Cmd {
		if name != mockCmd {
			return exec.Command(name, args...)
		}
		// Return a cmd-like object that uses our pipes.
		// We use a no-op command that we'll override the pipes on.
		cmd := exec.Command("echo") //nolint:gosec // test only
		cmd.Stdin = serverRead
		cmd.Stdout = serverWrite

		return cmd
	}

	client := &lspClient{
		config: LSPConfig{
			ServerCommand: mockCmd,
			Languages:     []Language{LangGo},
		},
	}

	// Manually set up the pipes (simulating what Initialize does after Start).
	client.stdin = clientWrite
	client.stdout = bufio.NewReaderSize(clientRead, 64*1024)

	// Run a mock server in the background that responds to requests.
	done := make(chan struct{})

	go func() {
		defer close(done)

		mockLSPServer(t, serverRead, serverWrite)
	}()

	// Test: send initialize request.
	client.mu.Lock()
	resp, err := client.callLocked("initialize", map[string]any{
		"processId": 1,
		"rootUri":   "file:///tmp/project",
	})
	client.mu.Unlock()

	if err != nil {
		t.Fatalf("initialize call: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("initialize error: %s", resp.Error.Message)
	}

	// Verify we got capabilities back.
	var initResult map[string]any
	if err := json.Unmarshal(resp.Result, &initResult); err != nil {
		t.Fatalf("unmarshal init result: %v", err)
	}

	if _, ok := initResult["capabilities"]; !ok {
		t.Error("expected capabilities in initialize result")
	}

	// Test: send documentSymbol request.
	client.mu.Lock()
	resp, err = client.callLocked("textDocument/documentSymbol", map[string]any{
		"textDocument": map[string]any{
			"uri": "file:///tmp/project/main.go",
		},
	})
	client.mu.Unlock()

	if err != nil {
		t.Fatalf("documentSymbol call: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("documentSymbol error: %s", resp.Error.Message)
	}

	var symbols []lspSymbolInformation
	if err := json.Unmarshal(resp.Result, &symbols); err != nil {
		t.Fatalf("unmarshal symbols: %v", err)
	}

	if len(symbols) != 2 {
		t.Errorf("got %d symbols, want 2", len(symbols))
	}

	if symbols[0].Name != "main" {
		t.Errorf("first symbol = %q, want %q", symbols[0].Name, "main")
	}

	// Test: send references request.
	client.mu.Lock()
	resp, err = client.callLocked("textDocument/references", map[string]any{
		"textDocument": map[string]any{
			"uri": "file:///tmp/project/main.go",
		},
		"position": map[string]any{
			"line":      0,
			"character": 0,
		},
	})
	client.mu.Unlock()

	if err != nil {
		t.Fatalf("references call: %v", err)
	}

	var locations []lspLocation
	if err := json.Unmarshal(resp.Result, &locations); err != nil {
		t.Fatalf("unmarshal references: %v", err)
	}

	if len(locations) != 1 {
		t.Errorf("got %d locations, want 1", len(locations))
	}

	// Test: send shutdown request.
	client.mu.Lock()
	resp, err = client.callLocked("shutdown", nil)
	client.mu.Unlock()

	if err != nil {
		t.Fatalf("shutdown call: %v", err)
	}

	if resp.Error != nil {
		t.Fatalf("shutdown error: %s", resp.Error.Message)
	}

	// Close pipes to clean up.
	_ = clientWrite.Close()
	_ = serverWrite.Close()

	<-done
}

// mockLSPServer reads JSON-RPC requests from r and writes responses to w.
// It handles: initialize, textDocument/documentSymbol, textDocument/references, shutdown.
func mockLSPServer(t *testing.T, r io.Reader, w io.Writer) {
	t.Helper()

	reader := bufio.NewReaderSize(r, 64*1024)

	for {
		// Read Content-Length header.
		var contentLength int

		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return // Connection closed.
			}

			line = strings.TrimSpace(line)
			if line == "" {
				break
			}

			if val, ok := strings.CutPrefix(line, "Content-Length:"); ok {
				val = strings.TrimSpace(val)

				n, err := strconv.Atoi(val)
				if err != nil {
					t.Errorf("mock server: invalid content-length: %v", err)
					return
				}

				contentLength = n
			}
		}

		if contentLength == 0 {
			continue
		}

		body := make([]byte, contentLength)
		if _, err := io.ReadFull(reader, body); err != nil {
			return
		}

		// Parse the request.
		var req struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.Number     `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}

		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("mock server: unmarshal request: %v", err)
			return
		}

		// If no ID, it's a notification (e.g., initialized, exit) — no response needed.
		if req.ID.String() == "" || req.ID.String() == "0" {
			if req.Method == "exit" {
				return
			}

			continue
		}

		id, _ := req.ID.Int64()

		var result any

		switch req.Method {
		case "initialize":
			result = map[string]any{
				"capabilities": map[string]any{
					"textDocumentSync":   1,
					"documentSymbolProvider": true,
					"referencesProvider":     true,
				},
			}

		case "textDocument/documentSymbol":
			result = []lspSymbolInformation{
				{
					Name: "main",
					Kind: 12, // Function
					Location: lspLocation{
						URI: "file:///tmp/project/main.go",
						Range: lspRange{
							Start: lspPosition{Line: 2, Character: 0},
							End:   lspPosition{Line: 2, Character: 10},
						},
					},
				},
				{
					Name: "Server",
					Kind: 23, // Struct
					Location: lspLocation{
						URI: "file:///tmp/project/main.go",
						Range: lspRange{
							Start: lspPosition{Line: 5, Character: 0},
							End:   lspPosition{Line: 5, Character: 15},
						},
					},
				},
			}

		case "textDocument/references":
			result = []lspLocation{
				{
					URI: "file:///tmp/project/other.go",
					Range: lspRange{
						Start: lspPosition{Line: 10, Character: 5},
						End:   lspPosition{Line: 10, Character: 15},
					},
				},
			}

		case "shutdown":
			result = nil

		default:
			// Unknown method.
			writeJSONRPCResponse(t, w, id, nil, &jsonrpcError{
				Code:    -32601,
				Message: "method not found: " + req.Method,
			})

			continue
		}

		writeJSONRPCResponse(t, w, id, result, nil)
	}
}

func writeJSONRPCResponse(t *testing.T, w io.Writer, id int64, result any, rpcErr *jsonrpcError) {
	t.Helper()

	resp := struct {
		JSONRPC string        `json:"jsonrpc"`
		ID      int64         `json:"id"`
		Result  any   `json:"result,omitempty"`
		Error   *jsonrpcError `json:"error,omitempty"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   rpcErr,
	}

	body, err := json.Marshal(resp)
	if err != nil {
		t.Errorf("mock server: marshal response: %v", err)
		return
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	if _, err := io.WriteString(w, header); err != nil {
		return // Pipe closed.
	}

	if _, err := w.Write(body); err != nil {
		return // Pipe closed.
	}
}
