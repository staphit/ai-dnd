package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"dndduet/internal/provider"
)

// AppServer keeps a single `codex app-server` process alive and runs
// schema-constrained DM turns over its persistent JSON-RPC/stdio connection,
// instead of spawning a fresh `codex exec` per request.
//
// Turns are serialised with a mutex: a local table resolves one DM turn at a
// time, so this keeps the event handling simple and race-free.
type AppServer struct {
	command string
	cwd     string

	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	incoming chan rpcMessage
	nextID   int
	started  bool
	initErr  error
}

type rpcMessage struct {
	ID     *int            `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// NewAppServer builds an AppServer wrapper (the process starts lazily on first
// turn).
func NewAppServer(command, cwd string) *AppServer {
	return &AppServer{command: command, cwd: cwd}
}

func (a *AppServer) send(method string, params any) (int, error) {
	a.nextID++
	id := a.nextID
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return 0, err
	}
	if _, err := a.stdin.Write(append(payload, '\n')); err != nil {
		return 0, err
	}
	return id, nil
}

func (a *AppServer) readLoop(stdout io.Reader) {
	dec := json.NewDecoder(stdout)
	for {
		var msg rpcMessage
		if err := dec.Decode(&msg); err != nil {
			close(a.incoming)
			return
		}
		a.incoming <- msg
	}
}

// ensureStarted spawns the process and performs the initialize handshake once.
// Caller must hold a.mu.
func (a *AppServer) ensureStarted() error {
	if a.started {
		return a.initErr
	}
	a.started = true

	cmd := exec.Command(a.command, "app-server")
	cmd.Env = execEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		a.initErr = err
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.initErr = err
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		a.initErr = errors.New("找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH")
		return a.initErr
	}
	a.cmd = cmd
	a.stdin = stdin
	a.incoming = make(chan rpcMessage, 256)
	go a.readLoop(stdout)

	id, err := a.send("initialize", map[string]any{"clientInfo": map[string]any{"name": "dnd-duet", "version": "0.1.0"}})
	if err != nil {
		a.initErr = err
		return err
	}
	if _, err := a.awaitResult(id, 30*time.Second); err != nil {
		a.initErr = fmt.Errorf("Codex app-server 初始化失敗：%w", err)
		return a.initErr
	}
	return nil
}

// awaitResult drains events until the response to id arrives.
func (a *AppServer) awaitResult(id int, timeout time.Duration) (json.RawMessage, error) {
	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-a.incoming:
			if !ok {
				return nil, errors.New("Codex app-server 連線已中斷")
			}
			if msg.ID != nil && *msg.ID == id {
				if len(msg.Error) > 0 {
					return nil, errors.New(string(msg.Error))
				}
				return msg.Result, nil
			}
		case <-deadline:
			return nil, errors.New("Codex app-server 回應逾時")
		}
	}
}

// RunStructuredTurn opens a fresh thread, runs one schema-constrained turn, and
// returns the final assistant message (the schema-conforming JSON).
func (a *AppServer) RunStructuredTurn(ctx context.Context, prompt, model, effort, schemaJSON string, timeout time.Duration) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureStarted(); err != nil {
		return nil, err
	}

	readOnly := map[string]any{"type": "readOnly"}
	threadReqID, err := a.send("thread/start", map[string]any{"cwd": a.cwd, "approvalPolicy": "never", "sandboxPolicy": readOnly})
	if err != nil {
		return nil, err
	}
	threadRes, err := a.awaitResult(threadReqID, 30*time.Second)
	if err != nil {
		return nil, err
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(threadRes, &tr); err != nil || tr.Thread.ID == "" {
		return nil, errors.New("Codex app-server 沒有回傳 thread 識別碼")
	}

	var schemaObj any
	if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err != nil {
		return nil, err
	}
	params := map[string]any{
		"threadId":       tr.Thread.ID,
		"input":          []any{map[string]any{"type": "text", "text": prompt}},
		"outputSchema":   schemaObj,
		"cwd":            a.cwd,
		"sandboxPolicy":  readOnly,
		"approvalPolicy": "never",
	}
	if model != "" {
		params["model"] = model
	}
	if effort != "" {
		params["effort"] = effort
	}
	turnReqID, err := a.send("turn/start", params)
	if err != nil {
		return nil, err
	}
	if _, err := a.awaitResult(turnReqID, 30*time.Second); err != nil {
		return nil, err
	}

	if timeout == 0 {
		timeout = 180 * time.Second
	}
	deadline := time.After(timeout)
	var finalText string
	for {
		select {
		case msg, ok := <-a.incoming:
			if !ok {
				return nil, errors.New("Codex app-server 連線已中斷")
			}
			switch msg.Method {
			case "item/completed":
				var pe struct {
					Item struct {
						Type string `json:"type"`
						Text string `json:"text"`
					} `json:"item"`
				}
				if json.Unmarshal(msg.Params, &pe) == nil && pe.Item.Type == "agentMessage" {
					finalText = pe.Item.Text
				}
			case "turn/completed":
				var pc struct {
					Turn struct {
						Status string          `json:"status"`
						Error  json.RawMessage `json:"error"`
					} `json:"turn"`
				}
				_ = json.Unmarshal(msg.Params, &pc)
				if pc.Turn.Status == "failed" {
					return nil, errors.New(appServerErrorMessage(pc.Turn.Error))
				}
				if finalText == "" {
					return nil, errors.New("Codex app-server 沒有回傳最終訊息")
				}
				return json.RawMessage(finalText), nil
			case "error":
				var pe struct {
					Error json.RawMessage `json:"error"`
				}
				_ = json.Unmarshal(msg.Params, &pe)
				return nil, errors.New(appServerErrorMessage(pe.Error))
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline:
			return nil, errors.New("Codex app-server 回應逾時")
		}
	}
}

// appServerErrorMessage pulls a human-readable message out of the error payload.
func appServerErrorMessage(raw json.RawMessage) string {
	if len(raw) == 0 {
		return "Codex app-server 回報錯誤"
	}
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(raw, &e) == nil && e.Message != "" {
		return e.Message
	}
	return string(raw)
}

// Close terminates the app-server process.
func (a *AppServer) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cmd != nil && a.cmd.Process != nil {
		return a.cmd.Process.Kill()
	}
	return nil
}

// AppServerClient adapts an AppServer to provider.API. It runs DM turns over the
// persistent connection and delegates everything else (image generation, status,
// model helpers) to the exec-based Client.
type AppServerClient struct {
	*Client
	server *AppServer
}

var _ provider.API = (*AppServerClient)(nil)

// NewAppServerClient builds a persistent-connection provider. cwd is the working
// directory passed to the app-server.
func NewAppServerClient(cwd string) *AppServerClient {
	base := NewClient()
	return &AppServerClient{Client: base, server: NewAppServer(base.Command, cwd)}
}

// RunStructured runs the turn over the persistent app-server connection. The
// model and effort are already resolved by handleDm, so they are used directly.
func (c *AppServerClient) RunStructured(ctx context.Context, prompt string, opts provider.StructuredOpts) (json.RawMessage, error) {
	schemaBytes, err := os.ReadFile(opts.SchemaPath)
	if err != nil {
		return nil, err
	}
	return c.server.RunStructuredTurn(ctx, prompt, opts.Model, opts.Effort, string(schemaBytes), opts.Timeout)
}

// Close releases the persistent process.
func (c *AppServerClient) Close() error { return c.server.Close() }
