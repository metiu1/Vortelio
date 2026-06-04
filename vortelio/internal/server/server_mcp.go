package server

import (
	"encoding/json"
	"net/http"

	"github.com/vortelio/vortelio/internal/mcp"
)

// GET  /api/mcp/servers          — list configured MCP servers + status + tools
// POST /api/mcp/servers          — add/replace a server (body: mcp.ServerConfig)
func handleMCPServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		respond(w, 200, map[string]interface{}{"servers": mcp.Default().List()})
	case http.MethodPost:
		var cfg mcp.ServerConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			jsonError(w, 400, "invalid JSON: "+err.Error())
			return
		}
		if err := mcp.Default().AddServer(cfg); err != nil {
			jsonError(w, 400, err.Error())
			return
		}
		respond(w, 200, map[string]interface{}{"status": "saved", "servers": mcp.Default().List()})
	default:
		jsonError(w, 405, "use GET or POST")
	}
}

// POST /api/mcp/enable  — {name, enabled}
func handleMCPEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if err := mcp.Default().SetEnabled(req.Name, req.Enabled); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	respond(w, 200, map[string]interface{}{"status": "ok", "servers": mcp.Default().List()})
}

// POST /api/mcp/remove  — {name}
func handleMCPRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "use POST")
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if err := mcp.Default().RemoveServer(req.Name); err != nil {
		jsonError(w, 400, err.Error())
		return
	}
	respond(w, 200, map[string]interface{}{"status": "ok", "servers": mcp.Default().List()})
}
