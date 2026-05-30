package firebase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"cloud.google.com/go/firestore"
	fb "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

var (
	once    sync.Once
	fbApp   *fb.App
	fbAuth  *auth.Client
	fbStore *firestore.Client
	initErr error
)

// Init initializes Firebase using the service account key from ~/.vortelio/firebase-credentials.json.
// Safe to call multiple times; initialization happens only once.
func Init() error {
	once.Do(func() {
		credPath := credentialsPath()
		if _, err := os.Stat(credPath); os.IsNotExist(err) {
			initErr = fmt.Errorf("firebase credentials not found at %s", credPath)
			return
		}
		ctx := context.Background()
		opt := option.WithCredentialsFile(credPath)
		fbApp, initErr = fb.NewApp(ctx, nil, opt)
		if initErr != nil {
			return
		}
		fbAuth, initErr = fbApp.Auth(ctx)
		if initErr != nil {
			return
		}
		fbStore, initErr = fbApp.Firestore(ctx)
	})
	return initErr
}

// Enabled reports whether Firebase was successfully initialized.
func Enabled() bool { return fbAuth != nil && fbStore != nil }

// Auth returns the Firebase Auth client. Panics if Init() was not called.
func Auth() *auth.Client { return fbAuth }

// Store returns the Firestore client. Panics if Init() was not called.
func Store() *firestore.Client { return fbStore }

func credentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vortelio", "firebase-credentials.json")
}
