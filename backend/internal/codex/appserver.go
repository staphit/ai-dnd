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
// schema-constrained DM turns over its persistent JSON-RPC/stdio connection.
//
// Per the "one story = one connection" model, the process holds exactly one
// codex thread at a time, bound to the current story. Establishing or rebinding
// that thread happens only through Connect, which the HTTP layer calls when the
// player has explicitly consented — a turn for an unbound/dead connection
// returns provider.ErrNeedsConsent instead of silently (re)connecting.
//
// mu serialises Connect and RunTurn so only one turn runs at a time (a single
// local table). Response/notification routing is handled by a demultiplexer
// (routeMu) so a turn only ever sees its own events — late events from a
// timed-out or abandoned turn find no sink and are dropped.
type AppServer struct {
	command string
	cwd     string

	// mu serialises Connect/RunTurn and guards stdin writes + cmd/stdin.
	mu    sync.Mutex
	cmd   *exec.Cmd
	stdin io.WriteCloser

	// stateMu guards the binding snapshot so ConnectionState can be read cheaply
	// without blocking on a long-running turn (which holds mu, not stateMu).
	stateMu    sync.Mutex
	started    bool   // process spawned and initialised
	gen        int    // increments per (re)start; fences a dead process's readLoop
	alive      bool   // a thread is bound and the connection is usable
	boundStory string // story id the current thread belongs to
	threadID   string // codex thread bound to boundStory

	// routeMu guards the JSON-RPC demultiplexer state.
	routeMu sync.Mutex
	nextID  int
	pending map[int]chan rpcMessage // request id -> response waiter
	sink    chan rpcMessage         // current turn's notification stream (nil outside a turn)
}

type rpcMessage struct {
	ID     *int            `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// NewAppServer builds an AppServer wrapper (the process starts on first Connect).
func NewAppServer(command, cwd string) *AppServer {
	return &AppServer{command: command, cwd: cwd, pending: map[int]chan rpcMessage{}}
}

// ---------------------------------------------------------------------------
// JSON-RPC demultiplexer

// send registers a response waiter, writes the request, and returns its id and
// channel. The caller must hold a.mu (so stdin writes serialise).
func (a *AppServer) send(method string, params any) (int, chan rpcMessage, error) {
	a.routeMu.Lock()
	a.nextID++
	id := a.nextID
	ch := make(chan rpcMessage, 1)
	a.pending[id] = ch
	a.routeMu.Unlock()

	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		a.dropPending(id)
		return 0, nil, err
	}
	if a.stdin == nil {
		a.dropPending(id)
		return 0, nil, errors.New("Codex app-server 連線已中斷")
	}
	if _, err := a.stdin.Write(append(payload, '\n')); err != nil {
		a.dropPending(id)
		return 0, nil, err
	}
	return id, ch, nil
}

func (a *AppServer) dropPending(id int) {
	a.routeMu.Lock()
	delete(a.pending, id)
	a.routeMu.Unlock()
}

// request sends a request and waits for its response, honouring ctx and timeout.
// The caller must hold a.mu.
func (a *AppServer) request(ctx context.Context, method string, params any, timeout time.Duration) (json.RawMessage, error) {
	id, ch, err := a.send(method, params)
	if err != nil {
		return nil, err
	}
	defer a.dropPending(id)
	select {
	case msg, ok := <-ch:
		if !ok {
			return nil, errors.New("Codex app-server 連線已中斷")
		}
		if len(msg.Error) > 0 {
			return nil, errors.New(appServerErrorMessage(msg.Error))
		}
		return msg.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, errors.New("Codex app-server 回應逾時")
	}
}

func (a *AppServer) setSink(ch chan rpcMessage) {
	a.routeMu.Lock()
	a.sink = ch
	a.routeMu.Unlock()
}

func (a *AppServer) clearSink(ch chan rpcMessage) {
	a.routeMu.Lock()
	if a.sink == ch {
		a.sink = nil
	}
	a.routeMu.Unlock()
}

// readLoop decodes JSON-RPC messages and routes them: responses (id, no method)
// to their pending waiter; notifications (method, no id) to the current turn's
// sink. On a decode error the process is considered dead: the binding is
// cleared (fenced by generation) and all waiters/sink are failed so callers
// unblock. It never spawns a replacement — reconnecting requires player consent.
func (a *AppServer) readLoop(stdout io.Reader, gen int) {
	dec := json.NewDecoder(stdout)
	for {
		var msg rpcMessage
		if err := dec.Decode(&msg); err != nil {
			a.handleProcessDeath(gen)
			return
		}
		switch {
		case msg.ID != nil && msg.Method == "":
			a.deliverResponse(*msg.ID, msg)
		case msg.Method != "":
			a.deliverNotification(msg)
		default:
			// Malformed / inbound request under approvalPolicy:never — ignore.
		}
	}
}

func (a *AppServer) deliverResponse(id int, msg rpcMessage) {
	a.routeMu.Lock()
	ch := a.pending[id]
	delete(a.pending, id)
	a.routeMu.Unlock()
	if ch != nil {
		ch <- msg // buffered (size 1) — never blocks the read loop
	}
}

func (a *AppServer) deliverNotification(msg rpcMessage) {
	a.routeMu.Lock()
	ch := a.sink
	a.routeMu.Unlock()
	if ch == nil {
		return // no active turn — drop (prevents cross-turn contamination)
	}
	select {
	case ch <- msg:
	default:
		// sink full: drop rather than block the read loop; the turn deadline is
		// the backstop.
	}
}

func (a *AppServer) handleProcessDeath(gen int) {
	a.stateMu.Lock()
	if a.gen == gen {
		a.alive = false
		a.started = false
		a.boundStory = ""
		a.threadID = ""
	}
	a.stateMu.Unlock()

	a.routeMu.Lock()
	for id, ch := range a.pending {
		close(ch)
		delete(a.pending, id)
	}
	if a.sink != nil {
		close(a.sink)
		a.sink = nil
	}
	a.routeMu.Unlock()
}

// ---------------------------------------------------------------------------
// Process lifecycle

// ensureProcess spawns and initialises the app-server if it is not running.
// Caller must hold a.mu. A live process is reused across stories (Connect just
// starts a new thread on it).
func (a *AppServer) ensureProcess() error {
	a.stateMu.Lock()
	started := a.started
	a.stateMu.Unlock()
	if started {
		return nil
	}

	// Reap a previously-dead process before replacing it.
	if a.cmd != nil {
		_ = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
		a.cmd = nil
	}

	cmd := exec.Command(a.command, "app-server")
	cmd.Env = execEnv()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return errors.New("找不到 Codex CLI；請先安裝 Codex，或設定 CODEX_CLI_PATH")
	}
	a.cmd = cmd
	a.stdin = stdin

	a.stateMu.Lock()
	a.gen++
	gen := a.gen
	a.stateMu.Unlock()

	// Fresh routing state for the new process generation.
	a.routeMu.Lock()
	a.pending = map[int]chan rpcMessage{}
	a.sink = nil
	a.routeMu.Unlock()

	go a.readLoop(stdout, gen)

	if _, err := a.request(context.Background(), "initialize", map[string]any{"clientInfo": map[string]any{"name": "dnd-duet", "version": "0.1.0"}}, 30*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		a.cmd = nil
		a.stdin = nil
		return fmt.Errorf("Codex app-server 初始化失敗：%w", err)
	}

	a.stateMu.Lock()
	a.started = true
	a.stateMu.Unlock()
	return nil
}

var readOnlySandbox = map[string]any{"type": "readOnly"}

// Connect (re)establishes the connection and binds a fresh codex thread to the
// given story. This is the only path that creates a connection; the HTTP layer
// calls it only after explicit player consent.
func (a *AppServer) Connect(ctx context.Context, storyID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if err := a.ensureProcess(); err != nil {
		return err
	}
	res, err := a.request(ctx, "thread/start", map[string]any{"cwd": a.cwd, "approvalPolicy": "never", "sandboxPolicy": readOnlySandbox}, 30*time.Second)
	if err != nil {
		return err
	}
	var tr struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(res, &tr); err != nil || tr.Thread.ID == "" {
		return errors.New("Codex app-server 沒有回傳 thread 識別碼")
	}

	a.stateMu.Lock()
	a.threadID = tr.Thread.ID
	a.boundStory = storyID
	a.alive = true
	a.stateMu.Unlock()
	return nil
}

// ConnectionState reports the current binding without taking mu, so it stays
// responsive while a turn is running.
func (a *AppServer) ConnectionState() provider.ConnState {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	alive := a.alive && a.started && a.threadID != "" && a.boundStory != ""
	return provider.ConnState{Alive: alive, StoryID: a.boundStory}
}

// ---------------------------------------------------------------------------
// Turn execution

// RunTurn runs one schema-constrained DM turn on the story's bound thread and
// returns the final assistant message. It never connects implicitly: an unbound
// or dead connection, or a request for a different story than the one bound,
// returns provider.ErrNeedsConsent.
func (a *AppServer) RunTurn(ctx context.Context, storyID, prompt, model, effort, schemaJSON string, timeout time.Duration) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.stateMu.Lock()
	ready := a.alive && a.started && a.threadID != "" && a.boundStory == storyID
	thread := a.threadID
	a.stateMu.Unlock()
	if !ready {
		return nil, provider.ErrNeedsConsent
	}

	var schemaObj any
	if err := json.Unmarshal([]byte(schemaJSON), &schemaObj); err != nil {
		return nil, err
	}
	params := map[string]any{
		"threadId":       thread,
		"input":          []any{map[string]any{"type": "text", "text": prompt}},
		"outputSchema":   schemaObj,
		"cwd":            a.cwd,
		"sandboxPolicy":  readOnlySandbox,
		"approvalPolicy": "never",
	}
	if model != "" {
		params["model"] = model
	}
	if effort != "" {
		params["effort"] = effort
	}

	// Register this turn's notification sink before sending turn/start so no
	// early item/turn events are lost.
	events := make(chan rpcMessage, 64)
	a.setSink(events)
	defer a.clearSink(events)

	if _, err := a.request(ctx, "turn/start", params, 30*time.Second); err != nil {
		return nil, err
	}

	if timeout == 0 {
		timeout = 180 * time.Second
	}
	deadline := time.After(timeout)
	var finalText string
	for {
		select {
		case msg, ok := <-events:
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
			a.interrupt(thread)
			return nil, ctx.Err()
		case <-deadline:
			a.interrupt(thread)
			return nil, errors.New("Codex app-server 回應逾時")
		}
	}
}

// interrupt best-effort asks codex to stop the given thread's turn. Caller holds
// a.mu. Fire-and-forget: the protocol may not support it, and correctness does
// not depend on it (the sink is unregistered regardless).
func (a *AppServer) interrupt(threadID string) {
	payload, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": "thread/interrupt", "params": map[string]any{"threadId": threadID}})
	if err != nil || a.stdin == nil {
		return
	}
	_, _ = a.stdin.Write(append(payload, '\n'))
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

// resetLocked terminates the current process and clears all connection state.
// Caller must hold a.mu.
func (a *AppServer) resetLocked() error {
	if a.stdin != nil {
		_ = a.stdin.Close()
	}
	var err error
	if a.cmd != nil && a.cmd.Process != nil {
		err = a.cmd.Process.Kill()
		_ = a.cmd.Wait()
	}
	a.cmd = nil
	a.stdin = nil

	a.stateMu.Lock()
	a.started = false
	a.alive = false
	a.boundStory = ""
	a.threadID = ""
	a.stateMu.Unlock()

	a.routeMu.Lock()
	for id, ch := range a.pending {
		close(ch)
		delete(a.pending, id)
	}
	if a.sink != nil {
		close(a.sink)
		a.sink = nil
	}
	a.nextID = 0
	a.routeMu.Unlock()
	return err
}

// Reset discards a failed app-server connection so the next turn starts clean.
func (a *AppServer) Reset() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.resetLocked()
}

// Close terminates the app-server process.
func (a *AppServer) Close() error { return a.Reset() }

// ---------------------------------------------------------------------------
// AppServerClient adapts an AppServer to provider.API. It runs DM turns over the
// persistent connection and delegates everything else (image generation, status,
// model helpers) to the exec-based Client.
type AppServerClient struct {
	*Client
	server *AppServer

	schemaMu    sync.Mutex
	schemaCache map[string][]byte
}

var _ provider.API = (*AppServerClient)(nil)

// NewAppServerClient builds a persistent-connection provider. cwd is the working
// directory passed to the app-server.
func NewAppServerClient(cwd string) *AppServerClient {
	base := NewClient()
	return &AppServerClient{Client: base, server: NewAppServer(base.Command, cwd), schemaCache: map[string][]byte{}}
}

// Connect establishes the persistent connection bound to storyID (consent path).
func (c *AppServerClient) Connect(ctx context.Context, storyID string) error {
	return c.server.Connect(ctx, storyID)
}

// ConnectionState reports the persistent connection's binding.
func (c *AppServerClient) ConnectionState() provider.ConnState { return c.server.ConnectionState() }

func (c *AppServerClient) schema(path string) ([]byte, error) {
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	if b, ok := c.schemaCache[path]; ok {
		return b, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	c.schemaCache[path] = b
	return b, nil
}

// RunStructured runs the turn over the persistent app-server connection. The
// model and effort are already resolved by handleDm, so they are used directly.
func (c *AppServerClient) RunStructured(ctx context.Context, prompt string, opts provider.StructuredOpts) (json.RawMessage, error) {
	schemaBytes, err := c.schema(opts.SchemaPath)
	if err != nil {
		return nil, err
	}
	return c.server.RunTurn(ctx, opts.StoryID, prompt, opts.Model, opts.Effort, string(schemaBytes), opts.Timeout)
}

// Close releases the persistent process.
func (c *AppServerClient) Close() error { return c.server.Close() }
