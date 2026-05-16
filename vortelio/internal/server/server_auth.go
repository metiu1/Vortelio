package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	fb "github.com/vortelio/vortelio/internal/firebase"
)

// ── Middleware: extract UID from Bearer token ─────────────────────────────────

func uidFromRequest(r *http.Request) string {
	if !fb.Enabled() {
		return ""
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	idToken := strings.TrimPrefix(auth, "Bearer ")
	t, err := fb.Auth().VerifyIDToken(context.Background(), idToken)
	if err != nil {
		return ""
	}
	return t.UID
}

// ── POST /api/auth/verify ─────────────────────────────────────────────────────
// Body: {"id_token":"..."}
// Verifies Firebase ID token, creates user in Firestore if new, returns profile.
func handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if !fb.Enabled() {
		respond(w, 503, map[string]string{"error": "firebase not configured"})
		return
	}
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IDToken == "" {
		respond(w, 400, map[string]string{"error": "id_token required"})
		return
	}

	ctx := r.Context()
	token, err := fb.Auth().VerifyIDToken(ctx, body.IDToken)
	if err != nil {
		respond(w, 401, map[string]string{"error": "invalid token"})
		return
	}

	email, _ := token.Claims["email"].(string)
	profile, err := fb.GetOrCreateUser(ctx, token.UID, email)
	if err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	respond(w, 200, profile)
}

// ── GET /api/user/profile ─────────────────────────────────────────────────────

func handleUserProfile(w http.ResponseWriter, r *http.Request) {
	uid := uidFromRequest(r)
	if uid == "" {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	profile, err := fb.GetUserProfile(r.Context(), uid)
	if err != nil {
		respond(w, 404, map[string]string{"error": "user not found"})
		return
	}
	respond(w, 200, profile)
}

// ── PUT /api/user/settings ────────────────────────────────────────────────────

func handleUserSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	uid := uidFromRequest(r)
	if uid == "" {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respond(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	if err := fb.UpdateUserSettings(r.Context(), uid, settings); err != nil {
		respond(w, 500, map[string]string{"error": err.Error()})
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}

// ── /api/chats  (GET list | POST save | DELETE /api/chats/{id}) ───────────────

func handleChats(w http.ResponseWriter, r *http.Request) {
	uid := uidFromRequest(r)
	if uid == "" {
		respond(w, 401, map[string]string{"error": "unauthorized"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		chats, err := fb.ListChats(r.Context(), uid)
		if err != nil {
			respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		if chats == nil {
			chats = []fb.ChatRecord{}
		}
		respond(w, 200, map[string]interface{}{"chats": chats})

	case http.MethodPost:
		var chat fb.ChatRecord
		json.NewDecoder(r.Body).Decode(&chat)
		if chat.Title == "" {
			chat.Title = "Chat " + time.Now().Format("2006-01-02 15:04")
		}
		id, err := fb.SaveChat(r.Context(), uid, chat)
		if err != nil {
			respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]string{"id": id})

	case http.MethodDelete:
		chatID := strings.TrimPrefix(r.URL.Path, "/api/chats/")
		chatID = strings.TrimPrefix(chatID, "/api/chats")
		if chatID == "" {
			respond(w, 400, map[string]string{"error": "chat id required"})
			return
		}
		if err := fb.DeleteChat(r.Context(), uid, chatID); err != nil {
			respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]string{"status": "deleted"})

	default:
		respond(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// ── GET /api/auth/status ──────────────────────────────────────────────────────
// Returns whether Firebase is configured (used by web UI to show/hide auth UI).

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]interface{}{
		"firebase_enabled": fb.Enabled(),
	})
}
