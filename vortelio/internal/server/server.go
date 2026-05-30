package server

import (
	"context"
	"embed"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/config"
	fb "github.com/vortelio/vortelio/internal/firebase"
	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/runtime"
	"github.com/vortelio/vortelio/internal/version"
)

//go:embed ui.html
var uiFS embed.FS

// InitLogger configures the global slog logger. Call once at startup.
// level: "debug", "info", "warn", "error"
func InitLogger(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))
}

// ── Shared state ─────────────────────────────────────────────────────────────

var (
	activePullsMu sync.Mutex
	activePulls   = map[string]context.CancelFunc{}
	cachedHW      *runtime.Hardware
	cachedHWOnce  sync.Once
)

func registerPull(model string, cancel context.CancelFunc) {
	activePullsMu.Lock()
	defer activePullsMu.Unlock()
	activePulls[model] = cancel
}
func unregisterPull(model string) {
	activePullsMu.Lock()
	defer activePullsMu.Unlock()
	delete(activePulls, model)
}
func cancelPull(model string) bool {
	activePullsMu.Lock()
	defer activePullsMu.Unlock()
	if fn, ok := activePulls[model]; ok {
		fn()
		delete(activePulls, model)
		return true
	}
	return false
}

func getHardware() *runtime.Hardware {
	cachedHWOnce.Do(func() { cachedHW = runtime.DetectHardware() })
	return cachedHW
}

// ── Types ─────────────────────────────────────────────────────────────────────

type ModelWithSize struct {
	*hub.Model
	SizeHuman string `json:"size_human"`
}

type GenerateRequest struct {
	Model        string          `json:"model"`
	Prompt       string          `json:"prompt"`
	System       string          `json:"system"`   // system prompt override
	Template     string          `json:"template"` // prompt template override
	Context      []int           `json:"context"`  // conversation context tokens
	Images       []string        `json:"images"`   // base64-encoded images
	Raw          bool            `json:"raw"`      // skip template wrapping
	Messages     []ChatMessage   `json:"messages"`
	InputFile    string          `json:"input_file"`
	OutputFile   string          `json:"output_file"`
	Steps        int             `json:"steps"`
	Stream       *bool           `json:"stream"` // nil = true (streaming default)
	ForceCPU     bool            `json:"force_cpu"`
	ContextSize  int             `json:"context_size"`
	Think        bool            `json:"think"`
	ToolsEnabled bool            `json:"tools_enabled"`
	Format       json.RawMessage `json:"format"`     // "json" or JSON schema
	Suffix       string          `json:"suffix"`     // FIM suffix
	KeepAlive    json.RawMessage `json:"keep_alive"` // duration string or seconds
	Options      json.RawMessage `json:"options"`    // Ollama-style generation params
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ── Router ────────────────────────────────────────────────────────────────────

func NewMux() *http.ServeMux {
	go getHardware() // warm up hardware detection
	go func() {
		if err := fb.Init(); err != nil {
			slog.Info("firebase disabled", "reason", err.Error())
		} else {
			slog.Info("firebase initialized", "project", "vortelio-3e7a8")
		}
	}()
	mux := http.NewServeMux()

	ca := func(h http.HandlerFunc) http.HandlerFunc { return withObservability(withCORS(withAuth(h))) }

	mux.HandleFunc("/", handleUI)
	mux.HandleFunc("/api/status", ca(handleStatus))
	mux.HandleFunc("/api/upload", ca(handleUpload))

	// Hub — models + download (pull rate limited)
	mux.HandleFunc("/api/models", ca(handleModels))
	mux.HandleFunc("/api/models/", ca(handleModelByName))
	mux.HandleFunc("/api/models/remove", ca(handleModelRemove))
	mux.HandleFunc("/api/models/rename", ca(handleModelRename))
	mux.HandleFunc("/api/models/info", ca(handleModelInfo))
	mux.HandleFunc("/api/models/mmproj", ca(handleModelMmProj))
	mux.HandleFunc("/api/pull", ca(withRateLimit(pullLimiter, handlePull)))
	mux.HandleFunc("/api/pull/cancel", ca(handlePullCancel))

	// Generate (rate limited)
	mux.HandleFunc("/api/generate", ca(withRateLimit(generateLimiter, handleGenerate)))

	// Agents
	mux.HandleFunc("/api/agents/proxy", ca(handleAgentProxy))
	mux.HandleFunc("/api/agents/check", ca(handleAgentCheck))
	mux.HandleFunc("/api/agents/catalog", ca(handleAgentCatalog))
	mux.HandleFunc("/api/agents/install", ca(handleAgentInstall))
	mux.HandleFunc("/api/agents/start", ca(handleAgentStart))
	mux.HandleFunc("/api/agents/stop", ca(handleAgentStop))
	mux.HandleFunc("/api/agents/uninstall", ca(handleAgentUninstall))
	mux.HandleFunc("/api/agents/health", ca(handleAgentHealth))
	mux.HandleFunc("/api/ollama/models", ca(handleOllamaModels))

	// CrewAI orchestration (legacy JSON CRUD)
	mux.HandleFunc("/api/crewai/crews", ca(handleCrewList))
	mux.HandleFunc("/api/crewai/crews/", ca(handleCrewDispatch))
	// CrewAI Studio proxy → Python server port 8500
	mux.HandleFunc("/api/crewai/studio/", ca(handleCrewStudioProxy))

	// Graceful shutdown (used by vortelio stop)
	mux.HandleFunc("/api/shutdown", handleShutdown)

	// History
	mux.HandleFunc("/api/history", ca(handleHistory))

	// ── Ollama-compatible API ─────────────────────────────────────────────────
	mux.HandleFunc("/api/version", ca(handleOllamaVersion))
	mux.HandleFunc("/api/ps", ca(handleOllamaPs))
	mux.HandleFunc("/api/tags", ca(handleOllamaTags))
	mux.HandleFunc("/api/show", ca(handleOllamaShow))
	mux.HandleFunc("/api/delete", ca(handleOllamaDelete))
	mux.HandleFunc("/api/chat", ca(withRateLimit(generateLimiter, handleOllamaChat)))
	mux.HandleFunc("/api/embed", ca(handleOllamaEmbed))
	mux.HandleFunc("/api/embeddings", ca(handleOllamaEmbeddings))
	mux.HandleFunc("/api/copy", ca(handleOllamaCopy))
	mux.HandleFunc("/api/create", ca(handleOllamaCreate))
	mux.HandleFunc("/api/push", ca(handleOllamaPush))
	mux.HandleFunc("/api/quantize", ca(handleOllamaQuantize))
	mux.HandleFunc("/api/blobs/", ca(handleOllamaBlobs))

	// ── OpenAI-compatible API ─────────────────────────────────────────────────
	mux.HandleFunc("/v1/models", ca(handleOpenAIModelByID))
	mux.HandleFunc("/v1/models/", ca(handleOpenAIModelByID))
	mux.HandleFunc("/v1/chat/completions", ca(withRateLimit(generateLimiter, handleOpenAIChatCompletions)))
	mux.HandleFunc("/v1/completions", ca(withRateLimit(generateLimiter, handleOpenAICompletions)))
	mux.HandleFunc("/v1/embeddings", ca(handleOpenAIEmbeddings))
	mux.HandleFunc("/v1/audio/transcriptions", ca(withRateLimit(generateLimiter, handleOpenAIAudioTranscriptions)))
	mux.HandleFunc("/v1/audio/translations", ca(withRateLimit(generateLimiter, handleOpenAIAudioTranslations)))
	mux.HandleFunc("/v1/audio/speech", ca(withRateLimit(generateLimiter, handleOpenAIAudioSpeech)))
	mux.HandleFunc("/v1/images/generations", ca(withRateLimit(generateLimiter, handleOpenAIImageGenerations)))

	// ── Advanced features (Vortelio-only) ────────────────────────────────────
	mux.HandleFunc("/api/route", ca(handleRoute))
	mux.HandleFunc("/api/compare", ca(withRateLimit(generateLimiter, handleCompare)))
	mux.HandleFunc("/api/structured", ca(withRateLimit(generateLimiter, handleStructured)))
	mux.HandleFunc("/api/summarize", ca(withRateLimit(generateLimiter, handleSummarize)))
	mux.HandleFunc("/api/think", ca(withRateLimit(generateLimiter, handleThink)))
	mux.HandleFunc("/api/gguf/inspect", ca(handleGGUFInspect))
	mux.HandleFunc("/api/hooks", ca(handleHooks))
	mux.HandleFunc("/api/hooks/", ca(handleHooks))
	mux.HandleFunc("/api/audit", ca(handleAudit))
	mux.HandleFunc("/api/rag/ingest", ca(handleRAGIngest))
	mux.HandleFunc("/api/rag/query", ca(handleRAGQuery))
	mux.HandleFunc("/api/import/ollama", ca(handleImportOllama))
	mux.HandleFunc("/api/config", ca(handleConfig))

	// ── Firebase Auth & user data ─────────────────────────────────────────────
	// /api/auth/verify is public (no Vortelio API key needed — client sends Firebase ID token)
	mux.HandleFunc("/api/auth/verify", withObservability(withCORS(withRateLimit(authLimiter, handleAuthVerify))))
	mux.HandleFunc("/api/auth/status", withObservability(withCORS(handleAuthStatus)))
	mux.HandleFunc("/api/user/profile", ca(handleUserProfile))
	mux.HandleFunc("/api/user/settings", ca(handleUserSettings))
	mux.HandleFunc("/api/user/apikeys", ca(handleAPIKeys))
	mux.HandleFunc("/api/user/apikeys/", ca(handleAPIKeys))
	mux.HandleFunc("/api/chats", ca(handleChats))
	mux.HandleFunc("/api/chats/", ca(handleChats))

	// ── Cloud proxy (OpenRouter) ──────────────────────────────────────────────
	mux.HandleFunc("/api/proxy/models", withObservability(withCORS(handleProxyModels)))
	mux.HandleFunc("/api/proxy/chat", withObservability(withCORS(withRateLimit(generateLimiter, handleProxyChat))))
	mux.HandleFunc("/api/proxy/usage", ca(handleProxyUsage))

	// ── Stripe payments ───────────────────────────────────────────────────────
	mux.HandleFunc("/api/stripe/checkout", ca(handleStripeCheckout))
	mux.HandleFunc("/api/stripe/webhook", withObservability(withCORS(handleStripeWebhook)))

	// Public observability/spec (no auth — required for monitoring tools)
	mux.HandleFunc("/metrics", withObservability(withCORS(handleMetrics)))
	mux.HandleFunc("/openapi.json", withObservability(withCORS(handleOpenAPI)))

	return mux
}

// ── Core handlers ─────────────────────────────────────────────────────────────

func handleUI(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api") {
		http.NotFound(w, r)
		return
	}
	data, err := uiFS.ReadFile("ui.html")
	if err != nil {
		respond(w, 200, map[string]string{"name": "vortelio", "version": version.Version})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

var shutdownCh = make(chan struct{}, 1)

// ShutdownCh returns the channel closed by POST /api/shutdown.
func ShutdownCh() <-chan struct{} { return shutdownCh }

func handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	respond(w, 200, map[string]string{"status": "shutting down"})
	select {
	case shutdownCh <- struct{}{}:
	default:
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	store := hub.NewModelStore()
	models, _ := store.List()
	respond(w, 200, map[string]interface{}{
		"name": "vortelio", "version": version.Version,
		"model_count": len(models), "hardware": getHardware().String(),
	})
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	r.ParseMultipartForm(100 << 20)
	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, 400, "no file: "+err.Error())
		return
	}
	defer file.Close()
	tmp, err := os.CreateTemp("", "vortelio-upload-*-"+header.Filename)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	defer tmp.Close()
	io.Copy(tmp, file)
	respond(w, 200, map[string]string{"path": tmp.Name()})
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func respond(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	if code != http.StatusOK {
		w.WriteHeader(code)
	}
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	if code >= 500 {
		slog.Error("server error", "code", code, "msg", msg)
	}
	respond(w, code, map[string]string{"error": msg})
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		origin := r.Header.Get("Origin")
		if origin != "" {
			cfg := config.Get()
			allowed := false
			for _, o := range cfg.AllowOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}
			if !allowed && (strings.Contains(origin, "localhost") || strings.Contains(origin, "127.0.0.1")) {
				allowed = true
			}
			if allowed {
				if len(cfg.AllowOrigins) == 1 && cfg.AllowOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Firebase-Token")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		h(w, r)
		slog.Debug("request", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start).Round(time.Millisecond))
	}
}

func withAuth(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := config.Get().APIKey
		if key == "" {
			h(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth == "Bearer "+key || r.URL.Query().Get("api_key") == key {
			h(w, r)
			return
		}
		w.Header().Set("WWW-Authenticate", `Bearer realm="vortelio"`)
		jsonError(w, 401, "unauthorized")
	}
}
