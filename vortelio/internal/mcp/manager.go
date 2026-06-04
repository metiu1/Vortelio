package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/vortelio/vortelio/internal/config"
	"github.com/vortelio/vortelio/internal/runtime"
)

// ServerConfig describes a configured MCP server.
type ServerConfig struct {
	Name      string   `json:"name"`
	Transport string   `json:"transport"` // "stdio" | "http"
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Env       []string `json:"env,omitempty"` // "KEY=value"
	URL       string   `json:"url,omitempty"`
	Enabled   bool     `json:"enabled"`
}

// liveServer is a connected server with its discovered tools.
type liveServer struct {
	cfg     ServerConfig
	client  *Client
	tools   []Tool
	status  string // "connected" | "error" | "disabled"
	lastErr string
}

// Manager owns the set of configured MCP servers and their connections.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*liveServer
	order   []string
}

var (
	defaultManager *Manager
	once           sync.Once
)

// Default returns the process-wide MCP manager, loading persisted config once.
func Default() *Manager {
	once.Do(func() {
		defaultManager = &Manager{servers: map[string]*liveServer{}}
		defaultManager.load()
	})
	return defaultManager
}

func configPath() string {
	return filepath.Join(config.HomeDir(), "mcp.json")
}

func (m *Manager) load() {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return
	}
	var cfgs []ServerConfig
	if json.Unmarshal(data, &cfgs) != nil {
		return
	}
	for _, c := range cfgs {
		m.servers[c.Name] = &liveServer{cfg: c, status: "disabled"}
		m.order = append(m.order, c.Name)
		if c.Enabled {
			m.connect(m.servers[c.Name])
		}
	}
}

func (m *Manager) save() error {
	var cfgs []ServerConfig
	for _, name := range m.order {
		if s, ok := m.servers[name]; ok {
			cfgs = append(cfgs, s.cfg)
		}
	}
	data, _ := json.MarshalIndent(cfgs, "", "  ")
	if err := os.MkdirAll(config.HomeDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0600)
}

// connect dials the server and caches its tools. Caller holds the write lock.
func (m *Manager) connect(s *liveServer) {
	if s.client != nil {
		s.client.Close()
		s.client = nil
	}
	var cl *Client
	var err error
	switch s.cfg.Transport {
	case "http":
		cl, err = DialHTTP(s.cfg.URL)
	default:
		cl, err = DialStdio(s.cfg.Command, s.cfg.Args, s.cfg.Env)
	}
	if err != nil {
		s.status = "error"
		s.lastErr = err.Error()
		return
	}
	tools, err := cl.ListTools()
	if err != nil {
		cl.Close()
		s.status = "error"
		s.lastErr = err.Error()
		return
	}
	s.client = cl
	s.tools = tools
	s.status = "connected"
	s.lastErr = ""
}

// AddServer adds (or replaces) a server config and connects if enabled.
func (m *Manager) AddServer(cfg ServerConfig) error {
	if cfg.Name == "" {
		return fmt.Errorf("server name is required")
	}
	if cfg.Transport == "" {
		if cfg.URL != "" {
			cfg.Transport = "http"
		} else {
			cfg.Transport = "stdio"
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.servers[cfg.Name]; !exists {
		m.order = append(m.order, cfg.Name)
	}
	s := &liveServer{cfg: cfg, status: "disabled"}
	m.servers[cfg.Name] = s
	if cfg.Enabled {
		m.connect(s)
	}
	return m.save()
}

// RemoveServer disconnects and forgets a server.
func (m *Manager) RemoveServer(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.servers[name]; ok {
		if s.client != nil {
			s.client.Close()
		}
		delete(m.servers, name)
	}
	for i, n := range m.order {
		if n == name {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}
	return m.save()
}

// SetEnabled enables/disables a server, connecting or disconnecting as needed.
func (m *Manager) SetEnabled(name string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.servers[name]
	if !ok {
		return fmt.Errorf("unknown server: %s", name)
	}
	s.cfg.Enabled = enabled
	if enabled {
		m.connect(s)
	} else {
		if s.client != nil {
			s.client.Close()
			s.client = nil
		}
		s.status = "disabled"
		s.tools = nil
	}
	return m.save()
}

// ServerStatus is a UI-facing snapshot of a configured server.
type ServerStatus struct {
	ServerConfig
	Status    string   `json:"status"`
	LastError string   `json:"last_error,omitempty"`
	ToolNames []string `json:"tool_names,omitempty"`
}

// List returns the status of all configured servers.
func (m *Manager) List() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []ServerStatus
	for _, name := range m.order {
		s := m.servers[name]
		names := make([]string, 0, len(s.tools))
		for _, t := range s.tools {
			names = append(names, t.Name)
		}
		sort.Strings(names)
		out = append(out, ServerStatus{
			ServerConfig: s.cfg,
			Status:       s.status,
			LastError:    s.lastErr,
			ToolNames:    names,
		})
	}
	return out
}

// ── ToolProvider integration ─────────────────────────────────────────────────

// toolKey builds the namespaced tool name exposed to the model.
func toolKey(server, tool string) string {
	return "mcp__" + sanitize(server) + "__" + tool
}

func sanitize(s string) string {
	return strings.NewReplacer(" ", "_", "-", "_", ".", "_").Replace(s)
}

// Provider returns a runtime.ToolProvider exposing all connected MCP tools.
func (m *Manager) Provider() runtime.ToolProvider {
	return &mcpProvider{m: m}
}

type mcpProvider struct{ m *Manager }

func (p *mcpProvider) Tools() []runtime.ToolDef {
	p.m.mu.RLock()
	defer p.m.mu.RUnlock()
	var defs []runtime.ToolDef
	for _, name := range p.m.order {
		s := p.m.servers[name]
		if s.status != "connected" {
			continue
		}
		for _, t := range s.tools {
			schema := t.InputSchema
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			defs = append(defs, runtime.ToolDef{
				Type: "function",
				Function: runtime.ToolFuncDef{
					Name:        toolKey(s.cfg.Name, t.Name),
					Description: t.Description,
					Parameters:  schema,
				},
			})
		}
	}
	return defs
}

func (p *mcpProvider) Execute(name, argsJSON string) (string, error) {
	p.m.mu.RLock()
	defer p.m.mu.RUnlock()
	for _, sname := range p.m.order {
		s := p.m.servers[sname]
		if s.status != "connected" || s.client == nil {
			continue
		}
		for _, t := range s.tools {
			if toolKey(s.cfg.Name, t.Name) == name {
				return s.client.CallTool(t.Name, json.RawMessage(argsJSON))
			}
		}
	}
	return "", fmt.Errorf("unknown MCP tool: %s", name)
}
