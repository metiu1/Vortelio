package commands

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/vortelio/vortelio/internal/config"
	"github.com/vortelio/vortelio/internal/hub"
	"github.com/vortelio/vortelio/internal/server"
)

// codePrefs is the small persisted state of the `vortelio code` session so the
// last-used model and mode are restored on the next launch.
type codePrefs struct {
	ModelName     string `json:"model_name"`
	ModelTag      string `json:"model_tag"`
	CloudProvider string `json:"cloud_provider"`
	CloudModel    string `json:"cloud_model"`
	Mode          string `json:"mode"`
}

func codePrefsPath() string {
	return filepath.Join(config.HomeDir(), "code_session.json")
}

func loadCodePrefs() codePrefs {
	var p codePrefs
	if data, err := os.ReadFile(codePrefsPath()); err == nil {
		_ = json.Unmarshal(data, &p)
	}
	return p
}

// savePrefs writes the current model/mode so the next session starts where this
// one left off. Best-effort: failures are silent (never block the session).
func (s *codeSession) savePrefs() {
	p := codePrefs{CloudProvider: s.cloudProvider, CloudModel: s.cloudModel, Mode: s.mode}
	if s.model != nil {
		p.ModelName, p.ModelTag = s.model.Name, s.model.Tag
	}
	_ = os.MkdirAll(config.HomeDir(), 0o755)
	if data, err := json.MarshalIndent(p, "", "  "); err == nil {
		_ = os.WriteFile(codePrefsPath(), data, 0o644)
	}
}

// restoreFromPrefs sets the session model/mode from the last session if that
// model is still available. Returns true if a model (local or cloud) was set.
func (s *codeSession) restoreFromPrefs(store *hub.ModelStore) bool {
	p := loadCodePrefs()
	if p.Mode == "plan" || p.Mode == "ask" || p.Mode == "auto" {
		s.mode = p.Mode
		s.autonomous = p.Mode == "auto"
	}
	if p.CloudProvider != "" && p.CloudModel != "" {
		for _, c := range server.CloudModelsForCLI() {
			if c.Provider == p.CloudProvider && c.Model == p.CloudModel {
				s.cloudProvider, s.cloudModel = p.CloudProvider, p.CloudModel
				return true
			}
		}
	}
	if p.ModelName != "" {
		if ref, err := hub.ParseModelRef(p.ModelName + ":" + p.ModelTag); err == nil {
			if m, err := store.Resolve(ref); err == nil {
				s.model = m
				return true
			}
		}
	}
	return false
}
