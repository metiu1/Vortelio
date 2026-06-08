package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/vortelio/vortelio/internal/updater"
)

func handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, 405, "GET only")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	info, err := updater.Check(ctx)
	if err != nil {
		jsonError(w, 502, err.Error())
		return
	}
	respond(w, 200, info)
}

func handleUpdateStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, 405, "POST only")
		return
	}
	var req struct {
		Force   bool `json:"force"`
		Restart bool `json:"restart"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	if !req.Force {
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		info, err := updater.Check(ctx)
		cancel()
		if err != nil {
			jsonError(w, 502, err.Error())
			return
		}
		if !info.Available {
			respond(w, 200, map[string]interface{}{"started": false, "message": "Vortelio e' gia' aggiornato.", "info": info})
			return
		}
	}

	res, err := updater.StartDetached(req.Restart)
	if err != nil {
		jsonError(w, 500, err.Error())
		return
	}
	respond(w, 200, res)
	go func() {
		time.Sleep(500 * time.Millisecond)
		select {
		case shutdownCh <- struct{}{}:
		default:
		}
	}()
}
