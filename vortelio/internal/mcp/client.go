// Package mcp implements a minimal Model Context Protocol (MCP) client.
// It supports two transports:
//   - stdio:  spawn a local command and exchange newline-delimited JSON-RPC.
//   - http:   POST JSON-RPC messages to a streamable-HTTP endpoint.
//
// Only the subset needed by Vortelio is implemented: initialize, tools/list,
// tools/call.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

const protocolVersion = "2024-11-05"

// Tool describes a tool exposed by an MCP server.
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// rpcRequest / rpcResponse model JSON-RPC 2.0 messages.
type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("mcp rpc error %d: %s", e.Code, e.Message) }

// transport sends a JSON-RPC request and returns the raw result, or sends a
// notification (no id, no response expected).
type transport interface {
	call(ctx context.Context, method string, params interface{}) (json.RawMessage, error)
	notify(method string, params interface{}) error
	close() error
}

// Client is a connected MCP client.
type Client struct {
	tr transport
}

// ── stdio transport ───────────────────────────────────────────────────────────

type stdioTransport struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	mu      sync.Mutex
	pending map[string]chan rpcResponse
	pmu     sync.Mutex
	nextID  int64
	closed  atomic.Bool
}

func newStdioTransport(command string, args, env []string) (*stdioTransport, error) {
	cmd := exec.Command(command, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server %q: %w", command, err)
	}
	t := &stdioTransport{cmd: cmd, stdin: stdin, pending: map[string]chan rpcResponse{}}
	go t.readLoop(stdout)
	return t, nil
}

func (t *stdioTransport) readLoop(stdout io.Reader) {
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var resp rpcResponse
		if json.Unmarshal(line, &resp) != nil || len(resp.ID) == 0 {
			continue // notification or non-response
		}
		id := string(bytes.Trim(resp.ID, `"`))
		t.pmu.Lock()
		ch, ok := t.pending[id]
		if ok {
			delete(t.pending, id)
		}
		t.pmu.Unlock()
		if ok {
			ch <- resp
		}
	}
}

func (t *stdioTransport) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	if t.closed.Load() {
		return nil, fmt.Errorf("mcp transport closed")
	}
	id := fmt.Sprintf("%d", atomic.AddInt64(&t.nextID, 1))
	ch := make(chan rpcResponse, 1)
	t.pmu.Lock()
	t.pending[id] = ch
	t.pmu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	if err := t.write(req); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		t.pmu.Lock()
		delete(t.pending, id)
		t.pmu.Unlock()
		return nil, ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return nil, resp.Error
		}
		return resp.Result, nil
	}
}

func (t *stdioTransport) notify(method string, params interface{}) error {
	return t.write(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
}

func (t *stdioTransport) write(req rpcRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	b = append(b, '\n')
	_, err = t.stdin.Write(b)
	return err
}

func (t *stdioTransport) close() error {
	t.closed.Store(true)
	_ = t.stdin.Close()
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Kill()
	}
	return nil
}

// ── http transport ──────────────────────────────────────────────────────────

type httpTransport struct {
	url    string
	client *http.Client
	nextID int64
}

func newHTTPTransport(url string) *httpTransport {
	return &httpTransport{url: url, client: &http.Client{Timeout: 60 * time.Second}}
}

func (t *httpTransport) call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	id := atomic.AddInt64(&t.nextID, 1)
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params})
	req, err := http.NewRequestWithContext(ctx, "POST", t.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024))
	// Streamable HTTP may wrap the JSON in SSE "data:" lines.
	raw = extractJSON(raw)
	var rr rpcResponse
	if err := json.Unmarshal(raw, &rr); err != nil {
		return nil, fmt.Errorf("invalid MCP response: %w", err)
	}
	if rr.Error != nil {
		return nil, rr.Error
	}
	return rr.Result, nil
}

func (t *httpTransport) notify(method string, params interface{}) error {
	body, _ := json.Marshal(rpcRequest{JSONRPC: "2.0", Method: method, Params: params})
	req, err := http.NewRequest("POST", t.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

func (t *httpTransport) close() error { return nil }

// extractJSON pulls a JSON-RPC object out of a possibly SSE-framed response body.
func extractJSON(raw []byte) []byte {
	raw = bytes.TrimSpace(raw)
	if len(raw) > 0 && raw[0] == '{' {
		return raw
	}
	// Look for the last "data: {...}" line (SSE framing).
	sc := bufio.NewScanner(bytes.NewReader(raw))
	sc.Buffer(make([]byte, 1024*1024), 8*1024*1024)
	var last []byte
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if bytes.HasPrefix(line, []byte("data:")) {
			candidate := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if len(candidate) > 0 && candidate[0] == '{' {
				last = append([]byte{}, candidate...)
			}
		}
	}
	if last != nil {
		return last
	}
	return raw
}

// ── client API ───────────────────────────────────────────────────────────────

// DialStdio connects to a stdio MCP server, runs the initialize handshake.
func DialStdio(command string, args, env []string) (*Client, error) {
	tr, err := newStdioTransport(command, args, env)
	if err != nil {
		return nil, err
	}
	c := &Client{tr: tr}
	if err := c.initialize(); err != nil {
		tr.close()
		return nil, err
	}
	return c, nil
}

// DialHTTP connects to an HTTP MCP server, runs the initialize handshake.
func DialHTTP(url string) (*Client, error) {
	tr := newHTTPTransport(url)
	c := &Client{tr: tr}
	if err := c.initialize(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) initialize() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := c.tr.call(ctx, "initialize", map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "vortelio", "version": "1.0"},
	})
	if err != nil {
		return err
	}
	return c.tr.notify("notifications/initialized", map[string]interface{}{})
}

// ListTools returns the tools exposed by the server.
func (c *Client) ListTools() ([]Tool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := c.tr.call(ctx, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var out struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return out.Tools, nil
}

// CallTool invokes a tool and returns the textual content of the result.
func (c *Client) CallTool(name string, args json.RawMessage) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var arguments interface{}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &arguments)
	}
	res, err := c.tr.call(ctx, "tools/call", map[string]interface{}{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		IsError bool `json:"isError"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return string(res), nil
	}
	var sb bytes.Buffer
	for _, ct := range out.Content {
		if ct.Text != "" {
			if sb.Len() > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(ct.Text)
		}
	}
	if out.IsError {
		return "", fmt.Errorf("tool error: %s", sb.String())
	}
	return sb.String(), nil
}

// Close shuts down the transport.
func (c *Client) Close() error { return c.tr.close() }
