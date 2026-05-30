package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/hub"
)

// ─────────────────────────────────────────────────────────────────────────────
// /api/import/ollama — import models from local Ollama installation
//
// Ollama stores manifests at ~/.ollama/models/manifests/<registry>/<ns>/<name>/<tag>
// and blobs at ~/.ollama/models/blobs/sha256-<digest>
//
// We register them in Vortelio's manifest store WITHOUT copying — local_path
// points to the existing Ollama blob, so disk usage doesn't double.
// ─────────────────────────────────────────────────────────────────────────────

type ollamaManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	MediaType     string `json:"mediaType"`
	Layers        []struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
		Size      int64  `json:"size"`
	} `json:"layers"`
	Config struct {
		Digest string `json:"digest"`
	} `json:"config"`
}

func handleImportOllama(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		OllamaPath string `json:"ollama_path"` // optional override
		DryRun     bool   `json:"dry_run"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	root := req.OllamaPath
	if root == "" {
		root = ollamaDefaultDir()
	}
	manifestsDir := filepath.Join(root, "models", "manifests")
	blobsDir := filepath.Join(root, "models", "blobs")

	if _, err := os.Stat(manifestsDir); err != nil {
		jsonError(w, 404, "Ollama installation not found at "+root)
		return
	}

	store := hub.NewModelStore()
	var imported, skipped []map[string]interface{}

	filepath.WalkDir(manifestsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		// path looks like: .../manifests/registry.ollama.ai/library/llama3/8b
		// The tag is the filename, the model is the parent dir
		rel, _ := filepath.Rel(manifestsDir, path)
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) < 3 {
			return nil
		}
		tag := parts[len(parts)-1]
		name := parts[len(parts)-2]

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var mf ollamaManifest
		if err := json.Unmarshal(data, &mf); err != nil {
			return nil
		}

		// Find the largest blob — that's the GGUF model file
		var modelDigest string
		var modelSize int64
		var mmprojDigest string
		for _, l := range mf.Layers {
			if strings.Contains(l.MediaType, "model") && l.Size > modelSize {
				modelDigest = l.Digest
				modelSize = l.Size
			}
			if strings.Contains(l.MediaType, "projector") || strings.Contains(l.MediaType, "mmproj") {
				mmprojDigest = l.Digest
			}
		}
		if modelDigest == "" {
			skipped = append(skipped, map[string]interface{}{"model": name + ":" + tag, "reason": "no model layer"})
			return nil
		}

		blobName := strings.ReplaceAll(modelDigest, ":", "-")
		blobPath := filepath.Join(blobsDir, blobName)
		if _, err := os.Stat(blobPath); err != nil {
			skipped = append(skipped, map[string]interface{}{"model": name + ":" + tag, "reason": "blob missing: " + blobName})
			return nil
		}

		ref := &hub.ModelRef{Type: "llm", Name: name, Tag: tag}
		if existing, _ := store.Resolve(ref); existing != nil {
			skipped = append(skipped, map[string]interface{}{"model": name + ":" + tag, "reason": "already installed"})
			return nil
		}

		entry := map[string]interface{}{
			"model": fmt.Sprintf("llm/%s:%s", name, tag),
			"size":  modelSize,
			"path":  blobPath,
		}
		if req.DryRun {
			imported = append(imported, entry)
			return nil
		}

		m := &hub.Model{
			Type:         "llm",
			Name:         name,
			Tag:          tag,
			Format:       "gguf",
			SizeBytes:    modelSize,
			LocalPath:    blobPath,
			Source:       "ollama-import:" + root,
			Capabilities: []string{"chat", "completion"},
			DownloadedAt: time.Now(),
		}
		if mmprojDigest != "" {
			mmName := strings.ReplaceAll(mmprojDigest, ":", "-")
			mmPath := filepath.Join(blobsDir, mmName)
			if _, err := os.Stat(mmPath); err == nil {
				m.MmProjPath = mmPath
				m.Capabilities = append(m.Capabilities, "vision")
			}
		}
		if err := store.Save(m); err != nil {
			skipped = append(skipped, map[string]interface{}{"model": name + ":" + tag, "reason": "save failed: " + err.Error()})
			return nil
		}
		imported = append(imported, entry)
		return nil
	})

	respond(w, 200, map[string]interface{}{
		"ollama_path": root,
		"dry_run":     req.DryRun,
		"imported":    imported,
		"skipped":     skipped,
		"count":       len(imported),
	})
}

func ollamaDefaultDir() string {
	if v := os.Getenv("OLLAMA_MODELS"); v != "" {
		return filepath.Dir(filepath.Dir(v)) // OLLAMA_MODELS points to .../models
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ollama")
}
