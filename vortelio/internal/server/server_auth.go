package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	fb "github.com/vortelio/vortelio/internal/firebase"
)

const (
	maxChatBodyBytes   = 2 << 20  // 2 MB max per chat save request
	maxSettingsBytes   = 64 << 10 // 64 KB max per settings update
	maxChatTitleLen    = 200      // characters
	maxMessagesPerChat = 2000
)

// ── Token extraction ──────────────────────────────────────────────────────────

// uidFromRequest extracts and verifies a Firebase ID token from the request,
// returning the UID or "" if missing/invalid.
func uidFromRequest(r *http.Request) string {
	if !fb.Enabled() {
		return ""
	}
	// Prefer dedicated header to avoid conflict with Vortelio API key
	idToken := r.Header.Get("X-Firebase-Token")
	if idToken == "" {
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			idToken = strings.TrimPrefix(auth, "Bearer ")
		}
	}
	if idToken == "" {
		return ""
	}
	// Use request context so verification is cancelled if the client disconnects
	t, err := fb.Auth().VerifyIDToken(r.Context(), idToken)
	if err != nil {
		return ""
	}
	return t.UID
}

// ── POST /api/auth/verify ─────────────────────────────────────────────────────

func handleAuthVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respond(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	if !fb.Enabled() {
		respond(w, 503, map[string]string{"error": "firebase not configured"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10) // 8 KB — token is always small
	var body struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IDToken == "" {
		respond(w, 400, map[string]string{"error": "id_token required"})
		return
	}

	token, err := fb.Auth().VerifyIDToken(r.Context(), body.IDToken)
	if err != nil {
		respond(w, 401, map[string]string{"error": "invalid token"})
		return
	}

	email, _ := token.Claims["email"].(string)
	profile, err := fb.GetOrCreateUser(r.Context(), token.UID, email)
	if err != nil {
		slog.Error("firestore get/create user", "uid", token.UID, "err", err)
		respond(w, 500, map[string]string{"error": "internal error"})
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
		slog.Error("firestore get user profile", "uid", uid, "err", err)
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
	r.Body = http.MaxBytesReader(w, r.Body, maxSettingsBytes)
	var settings map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		respond(w, 400, map[string]string{"error": "invalid body"})
		return
	}
	// Whitelist allowed settings keys to prevent arbitrary Firestore writes
	allowed := map[string]bool{
		"theme": true, "language": true, "fontSize": true,
		"streamingEnabled": true, "contextSize": true, "systemPrompt": true,
		"defaultModel": true, "showThinking": true, "outputDir": true,
	}
	cleaned := make(map[string]interface{}, len(allowed))
	for k, v := range settings {
		if allowed[k] {
			cleaned[k] = v
		}
	}
	if err := fb.UpdateUserSettings(r.Context(), uid, cleaned); err != nil {
		slog.Error("firestore update settings", "uid", uid, "err", err)
		respond(w, 500, map[string]string{"error": "internal error"})
		return
	}
	respond(w, 200, map[string]string{"status": "ok"})
}

// ── /api/chats ────────────────────────────────────────────────────────────────

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
			slog.Error("firestore list chats", "uid", uid, "err", err)
			respond(w, 500, map[string]string{"error": "internal error"})
			return
		}
		if chats == nil {
			chats = []fb.ChatRecord{}
		}
		respond(w, 200, map[string]interface{}{"chats": chats})

	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, maxChatBodyBytes)
		var chat fb.ChatRecord
		if err := json.NewDecoder(r.Body).Decode(&chat); err != nil {
			respond(w, 400, map[string]string{"error": "invalid body"})
			return
		}
		// Sanitize title
		chat.Title = strings.TrimSpace(chat.Title)
		if chat.Title == "" {
			chat.Title = "Chat " + time.Now().Format("2006-01-02 15:04")
		}
		if utf8.RuneCountInString(chat.Title) > maxChatTitleLen {
			chat.Title = string([]rune(chat.Title)[:maxChatTitleLen])
		}
		// Cap model name length
		if len(chat.Model) > 200 {
			chat.Model = chat.Model[:200]
		}
		// Cap messages count
		if len(chat.Messages) > maxMessagesPerChat {
			chat.Messages = chat.Messages[:maxMessagesPerChat]
		}
		id, err := fb.SaveChat(r.Context(), uid, chat)
		if err != nil {
			slog.Error("firestore save chat", "uid", uid, "err", err)
			respond(w, 500, map[string]string{"error": "internal error"})
			return
		}
		respond(w, 200, map[string]string{"id": id})

	case http.MethodDelete:
		chatID := strings.TrimPrefix(r.URL.Path, "/api/chats/")
		chatID = strings.Trim(chatID, "/")
		// Validate chatID: Firestore auto-IDs are 20 alphanumeric chars
		if chatID == "" || len(chatID) > 128 || strings.ContainsAny(chatID, "/\\..") {
			respond(w, 400, map[string]string{"error": "invalid chat id"})
			return
		}
		if err := fb.DeleteChat(r.Context(), uid, chatID); err != nil {
			slog.Error("firestore delete chat", "uid", uid, "chatID", chatID, "err", err)
			respond(w, 500, map[string]string{"error": "internal error"})
			return
		}
		respond(w, 200, map[string]string{"status": "deleted"})

	default:
		respond(w, 405, map[string]string{"error": "method not allowed"})
	}
}

// ── GET /api/auth/status ──────────────────────────────────────────────────────

func handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	respond(w, 200, map[string]interface{}{
		"firebase_enabled": fb.Enabled(),
	})
}
