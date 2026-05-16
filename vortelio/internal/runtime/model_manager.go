package runtime

import (
	"fmt"
	"sync"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
)

// DefaultKeepAliveDuration is how long a loaded model stays alive after last use.
const DefaultKeepAliveDuration = 5 * time.Minute

// ModelManager keeps llama-server processes alive between HTTP requests.
type ModelManager struct {
	mu      sync.Mutex
	entries map[string]*managedEntry
}

type managedEntry struct {
	runner    *LLMRunner
	expiresAt time.Time // zero = never expire
}

// LoadedModel is returned by ListLoaded.
type LoadedModel struct {
	Model     *hub.Model
	ExpiresAt time.Time
	SizeVRAM  int64
}

// GlobalModelManager is the singleton used by the HTTP server.
var GlobalModelManager = newModelManager()

func newModelManager() *ModelManager {
	m := &ModelManager{entries: make(map[string]*managedEntry)}
	go m.evictLoop()
	return m
}

func modelKey(model *hub.Model) string {
	return fmt.Sprintf("%s/%s:%s", model.Type, model.Name, model.Tag)
}

// GetOrLoad returns a running LLMRunner, starting llama-server if needed.
// keepAlive = 0 → DefaultKeepAliveDuration; keepAlive < 0 → never expire.
func (m *ModelManager) GetOrLoad(model *hub.Model, hw *Hardware, keepAlive time.Duration) (*LLMRunner, error) {
	if keepAlive == 0 {
		keepAlive = DefaultKeepAliveDuration
	}
	key := modelKey(model)

	m.mu.Lock()
	if e, ok := m.entries[key]; ok {
		if keepAlive > 0 {
			e.expiresAt = time.Now().Add(keepAlive)
		} else {
			e.expiresAt = time.Time{}
		}
		runner := e.runner
		m.mu.Unlock()
		return runner, nil
	}
	m.mu.Unlock()

	runner := NewLLMRunnerForServer(model, hw)
	if err := runner.EnsureServer(); err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok {
		// Another goroutine loaded the same model concurrently
		runner.stopServer()
		if keepAlive > 0 {
			e.expiresAt = time.Now().Add(keepAlive)
		}
		return e.runner, nil
	}
	expiry := time.Time{}
	if keepAlive > 0 {
		expiry = time.Now().Add(keepAlive)
	}
	m.entries[key] = &managedEntry{runner: runner, expiresAt: expiry}
	return runner, nil
}

// Unload stops a model and removes it from the manager.
func (m *ModelManager) Unload(model *hub.Model) {
	key := modelKey(model)
	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.entries[key]; ok {
		e.runner.stopServer()
		delete(m.entries, key)
	}
}

// ListLoaded returns all currently loaded models with expiry info.
func (m *ModelManager) ListLoaded() []LoadedModel {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []LoadedModel
	for _, e := range m.entries {
		result = append(result, LoadedModel{
			Model:     e.runner.model,
			ExpiresAt: e.expiresAt,
		})
	}
	return result
}

func (m *ModelManager) evictLoop() {
	ticker := time.NewTicker(30 * time.Second)
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for key, e := range m.entries {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				e.runner.stopServer()
				delete(m.entries, key)
			}
		}
		m.mu.Unlock()
	}
}
