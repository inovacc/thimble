package analysis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// LSPClient defines the interface for Language Server Protocol interactions.
// Implementations communicate with an LSP server to extract symbols and references.
type LSPClient interface {
	// Initialize starts the LSP server and performs the initialize handshake.
	Initialize(rootDir string) error

	// Symbols returns document symbols for the given file path.
	Symbols(path string) ([]Symbol, error)

	// References returns all references at the given file and line.
	References(path string, line int) ([]Reference, error)

	// Shutdown gracefully terminates the LSP server.
	Shutdown() error
}

// LSPConfig holds the configuration for spawning an LSP server subprocess.
type LSPConfig struct {
	ServerCommand string     `json:"server_command"`
	ServerArgs    []string   `json:"server_args"`
	Languages     []Language `json:"languages"`
}

// SpawnFunc is the function used to create an LSP server subprocess.
// It can be replaced in tests to avoid spawning real processes.
var SpawnFunc = defaultSpawn

func defaultSpawn(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

// NewLSPClient creates an LSP client that spawns the configured server as a subprocess
// and communicates via JSON-RPC 2.0 over stdio.
func NewLSPClient(config LSPConfig) (LSPClient, error) {
	if config.ServerCommand == "" {
		return nil, fmt.Errorf("lsp: server command is required")
	}

	return &lspClient{
		config: config,
	}, nil
}

// lspClient implements LSPClient using a subprocess and JSON-RPC over stdio.
type lspClient struct {
	config  LSPConfig
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	mu      sync.Mutex
	nextID  atomic.Int64
	rootDir string
}

// jsonrpcRequest represents a JSON-RPC 2.0 request.
type jsonrpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int64       `json:"id"`
	Method  string      `json:"method"`
	Params  any `json:"params,omitempty"`
}

// jsonrpcResponse represents a JSON-RPC 2.0 response.
type jsonrpcResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int64            `json:"id"`
	Result  json.RawMessage  `json:"result,omitempty"`
	Error   *jsonrpcError    `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC 2.0 error object.
type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// jsonrpcNotification represents a JSON-RPC 2.0 notification (no id).
type jsonrpcNotification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  any `json:"params,omitempty"`
}

func (c *lspClient) Initialize(rootDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.rootDir = rootDir

	cmd := SpawnFunc(c.config.ServerCommand, c.config.ServerArgs...)
	cmd.Stderr = io.Discard

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("lsp: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("lsp: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("lsp: start %s: %w", c.config.ServerCommand, err)
	}

	c.cmd = cmd
	c.stdin = stdin
	c.stdout = bufio.NewReaderSize(stdout, 64*1024)

	// Send initialize request.
	rootURI := "file:///" + strings.ReplaceAll(rootDir, "\\", "/")

	params := map[string]any{
		"processId": os.Getpid(),
		"rootUri":   rootURI,
		"capabilities": map[string]any{
			"textDocument": map[string]any{
				"documentSymbol": map[string]any{
					"hierarchicalDocumentSymbolSupport": true,
				},
			},
		},
	}

	resp, err := c.call("initialize", params)
	if err != nil {
		return fmt.Errorf("lsp: initialize: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("lsp: initialize error: %s", resp.Error.Message)
	}

	// Send initialized notification.
	return c.notify("initialized", map[string]any{})
}

func (c *lspClient) Symbols(path string) ([]Symbol, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return nil, fmt.Errorf("lsp: client not initialized")
	}

	uri := pathToURI(path)

	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
	}

	resp, err := c.call("textDocument/documentSymbol", params)
	if err != nil {
		return nil, fmt.Errorf("lsp: documentSymbol: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("lsp: documentSymbol error: %s", resp.Error.Message)
	}

	// Parse LSP SymbolInformation or DocumentSymbol response.
	var lspSymbols []lspSymbolInformation
	if err := json.Unmarshal(resp.Result, &lspSymbols); err != nil {
		return nil, fmt.Errorf("lsp: unmarshal symbols: %w", err)
	}

	symbols := make([]Symbol, 0, len(lspSymbols))
	for _, ls := range lspSymbols {
		symbols = append(symbols, Symbol{
			Name:     ls.Name,
			Kind:     lspSymbolKindToKind(ls.Kind),
			File:     path,
			Line:     ls.Location.Range.Start.Line + 1, // LSP uses 0-based lines.
			Exported: len(ls.Name) > 0 && ls.Name[0] >= 'A' && ls.Name[0] <= 'Z',
		})
	}

	return symbols, nil
}

func (c *lspClient) References(path string, line int) ([]Reference, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return nil, fmt.Errorf("lsp: client not initialized")
	}

	uri := pathToURI(path)

	params := map[string]any{
		"textDocument": map[string]any{
			"uri": uri,
		},
		"position": map[string]any{
			"line":      line - 1, // Convert to 0-based.
			"character": 0,
		},
		"context": map[string]any{
			"includeDeclaration": true,
		},
	}

	resp, err := c.call("textDocument/references", params)
	if err != nil {
		return nil, fmt.Errorf("lsp: references: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("lsp: references error: %s", resp.Error.Message)
	}

	var locations []lspLocation
	if err := json.Unmarshal(resp.Result, &locations); err != nil {
		return nil, fmt.Errorf("lsp: unmarshal references: %w", err)
	}

	refs := make([]Reference, 0, len(locations))
	for _, loc := range locations {
		refs = append(refs, Reference{
			File: uriToPath(loc.URI),
			Line: loc.Range.Start.Line + 1,
			Kind: "reference",
		})
	}

	return refs, nil
}

func (c *lspClient) Shutdown() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		return nil
	}

	// Send shutdown request.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		resp, err := c.callLocked("shutdown", nil)
		if err != nil {
			done <- err
			return
		}

		if resp.Error != nil {
			done <- fmt.Errorf("lsp: shutdown error: %s", resp.Error.Message)
			return
		}

		// Send exit notification.
		done <- c.notifyLocked("exit", nil)
	}()

	select {
	case err := <-done:
		_ = c.stdin.Close()
		_ = c.cmd.Wait()

		return err
	case <-ctx.Done():
		_ = c.stdin.Close()

		if c.cmd.Process != nil {
			_ = c.cmd.Process.Kill()
		}

		_ = c.cmd.Wait()

		return nil
	}
}

// call sends a JSON-RPC request and waits for the response.
// Must be called with c.mu held.
func (c *lspClient) call(method string, params any) (*jsonrpcResponse, error) {
	return c.callLocked(method, params)
}

// callLocked is the actual implementation; assumes lock is already held.
func (c *lspClient) callLocked(method string, params any) (*jsonrpcResponse, error) {
	id := c.nextID.Add(1)

	req := jsonrpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.writeMessage(req); err != nil {
		return nil, err
	}

	return c.readResponse(id)
}

// notify sends a JSON-RPC notification (no response expected).
// Must be called with c.mu held.
func (c *lspClient) notify(method string, params any) error {
	return c.notifyLocked(method, params)
}

// notifyLocked sends a notification; assumes lock is already held.
func (c *lspClient) notifyLocked(method string, params any) error {
	notif := jsonrpcNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}

	return c.writeMessage(notif)
}

// writeMessage encodes and writes an LSP message with Content-Length header.
func (c *lspClient) writeMessage(msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("lsp: marshal: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	if _, err := io.WriteString(c.stdin, header); err != nil {
		return fmt.Errorf("lsp: write header: %w", err)
	}

	if _, err := c.stdin.Write(body); err != nil {
		return fmt.Errorf("lsp: write body: %w", err)
	}

	return nil
}

// readResponse reads LSP messages until we find the response matching the given id.
// Notifications and other messages are discarded.
func (c *lspClient) readResponse(id int64) (*jsonrpcResponse, error) {
	for {
		body, err := c.readMessage()
		if err != nil {
			return nil, err
		}

		var resp jsonrpcResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			// Could be a notification; skip it.
			continue
		}

		if resp.ID == id {
			return &resp, nil
		}

		// Not our response (notification or different request), keep reading.
	}
}

// readMessage reads a single LSP message (Content-Length header + body).
func (c *lspClient) readMessage() ([]byte, error) {
	var contentLength int

	// Read headers.
	for {
		line, err := c.stdout.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("lsp: read header: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers.
		}

		if val, ok := strings.CutPrefix(line, "Content-Length:"); ok {
			n, err := strconv.Atoi(strings.TrimSpace(val))
			if err != nil {
				return nil, fmt.Errorf("lsp: invalid content-length %q: %w", val, err)
			}

			contentLength = n
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("lsp: missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(c.stdout, body); err != nil {
		return nil, fmt.Errorf("lsp: read body: %w", err)
	}

	return body, nil
}

// LSP protocol types for parsing responses.

type lspSymbolInformation struct {
	Name     string      `json:"name"`
	Kind     int         `json:"kind"`
	Location lspLocation `json:"location"`
}

type lspLocation struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// lspSymbolKindToKind maps LSP SymbolKind numbers to our SymbolKind.
// See: https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#symbolKind
func lspSymbolKindToKind(kind int) SymbolKind {
	switch kind {
	case 5: // Class
		return KindStruct
	case 6: // Method
		return KindMethod
	case 11: // Interface
		return KindInterface
	case 12: // Function
		return KindFunction
	case 13: // Variable
		return KindVariable
	case 14: // Constant
		return KindConstant
	case 23: // Struct
		return KindStruct
	default:
		return KindVariable
	}
}

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	// Normalize to forward slashes.
	path = strings.ReplaceAll(path, "\\", "/")

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return "file://" + path
}

// uriToPath converts a file:// URI back to a filesystem path.
func uriToPath(uri string) string {
	path := strings.TrimPrefix(uri, "file://")

	// On Windows, strip leading slash before drive letter (e.g. /C:/foo -> C:/foo).
	if len(path) > 2 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return path
}

// lspEnabled checks if LSP is enabled for a specific language via env vars.
func lspEnabled(lang Language) bool {
	switch lang {
	case LangGo:
		return os.Getenv("THIMBLE_LSP_GOPLS") == "1"
	case LangTypeScript, LangPython, LangRust, LangProto, LangC, LangJava, LangShell:
		return false
	default:
		return false
	}
}

// defaultLSPConfig returns the default LSP configuration for a language, if available.
func defaultLSPConfig(lang Language) *LSPConfig {
	switch lang {
	case LangGo:
		return &LSPConfig{
			ServerCommand: "gopls",
			Languages:     []Language{LangGo},
		}
	case LangTypeScript, LangPython, LangRust, LangProto, LangC, LangJava, LangShell:
		return nil
	default:
		return nil
	}
}
