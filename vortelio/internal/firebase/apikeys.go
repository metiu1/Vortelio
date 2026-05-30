package firebase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// APIKey represents a user-facing Vortelio API key.
// The raw key is shown once at creation; only the hash is stored.
type APIKey struct {
	ID        string    `firestore:"id"         json:"id"`
	Name      string    `firestore:"name"       json:"name"`
	KeyHash   string    `firestore:"key_hash"   json:"-"`          // SHA-256 of raw key — never returned
	KeyPrefix string    `firestore:"key_prefix" json:"key_prefix"` // first 8 chars for display
	CreatedAt time.Time `firestore:"created_at" json:"created_at"`
	LastUsed  time.Time `firestore:"last_used"  json:"last_used,omitempty"`
}

// APIKeyWithRaw is returned only on creation — raw key shown once, then discarded.
type APIKeyWithRaw struct {
	APIKey
	RawKey string `json:"key"`
}

// GenerateAPIKey creates a new API key for the user, stores its hash in Firestore,
// and returns the raw key (shown once only).
func GenerateAPIKey(ctx context.Context, uid, name string) (*APIKeyWithRaw, error) {
	raw, err := generateRawKey()
	if err != nil {
		return nil, err
	}
	hash := hashKey(raw)
	prefix := raw[:12] // "vt_live_xxxx"

	ref := fbStore.Collection("users").Doc(uid).Collection("api_keys").NewDoc()
	rec := APIKey{
		ID:        ref.ID,
		Name:      name,
		KeyHash:   hash,
		KeyPrefix: prefix,
		CreatedAt: time.Now(),
	}
	if _, err := ref.Set(ctx, rec); err != nil {
		return nil, err
	}
	return &APIKeyWithRaw{APIKey: rec, RawKey: raw}, nil
}

// ListAPIKeys returns all API keys for a user (without hashes).
func ListAPIKeys(ctx context.Context, uid string) ([]APIKey, error) {
	iter := fbStore.Collection("users").Doc(uid).Collection("api_keys").
		OrderBy("created_at", firestore.Desc).Documents(ctx)
	defer iter.Stop()
	var keys []APIKey
	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var k APIKey
		doc.DataTo(&k)
		k.ID = doc.Ref.ID
		k.KeyHash = "" // never leak hash
		keys = append(keys, k)
	}
	return keys, nil
}

// RevokeAPIKey deletes an API key by its doc ID.
func RevokeAPIKey(ctx context.Context, uid, keyID string) error {
	_, err := fbStore.Collection("users").Doc(uid).Collection("api_keys").Doc(keyID).Delete(ctx)
	return err
}

// LookupAPIKey finds the user UID and profile by raw key value.
// Scans all users is expensive — instead we maintain a global index.
func LookupAPIKey(ctx context.Context, rawKey string) (uid string, profile *UserProfile, err error) {
	hash := hashKey(rawKey)
	// Query the global key index: collection "api_key_index" doc == hash
	snap, err := fbStore.Collection("api_key_index").Doc(hash).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return "", nil, nil // key not found — not an error
		}
		return "", nil, err
	}
	var idx struct {
		UID string `firestore:"uid"`
	}
	if err := snap.DataTo(&idx); err != nil {
		return "", nil, err
	}
	// Update last_used async
	go func() {
		fbStore.Collection("api_key_index").Doc(hash).Update(context.Background(), []firestore.Update{
			{Path: "last_used", Value: time.Now()},
		})
	}()
	p, err := GetUserProfile(ctx, idx.UID)
	return idx.UID, p, err
}

// RegisterKeyIndex writes the global lookup index entry when a key is created.
// Called by GenerateAPIKey — kept separate so the index can be rebuilt if needed.
func RegisterKeyIndex(ctx context.Context, uid, rawKey string) error {
	hash := hashKey(rawKey)
	_, err := fbStore.Collection("api_key_index").Doc(hash).Set(ctx, map[string]interface{}{
		"uid":        uid,
		"created_at": time.Now(),
	})
	return err
}

// ── helpers ───────────────────────────────────────────────────────────────────

func generateRawKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return "vt_live_" + hex.EncodeToString(b), nil
}

func hashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
