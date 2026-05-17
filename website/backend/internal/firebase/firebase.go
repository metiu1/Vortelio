package firebase

import (
	"context"
	"fmt"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
	"github.com/vortelio/website/internal/config"
)

var (
	once    sync.Once
	fbAuth  *auth.Client
	fbStore *firestore.Client
	initErr error
)

func Init() error {
	once.Do(func() {
		ctx := context.Background()
		opt := option.WithCredentialsFile(config.Get().FirebaseCredentials)
		app, err := firebase.NewApp(ctx, nil, opt)
		if err != nil {
			initErr = fmt.Errorf("firebase app: %w", err)
			return
		}
		fbAuth, err = app.Auth(ctx)
		if err != nil {
			initErr = fmt.Errorf("firebase auth: %w", err)
			return
		}
		fbStore, err = app.Firestore(ctx)
		if err != nil {
			initErr = fmt.Errorf("firestore: %w", err)
		}
	})
	return initErr
}

// UpdateUserPlan sets the plan field on the user's Firestore profile.
func UpdateUserPlan(ctx context.Context, uid, plan string) error {
	_, err := fbStore.Collection("users").Doc(uid).Set(ctx,
		map[string]interface{}{"plan": plan},
		firestore.MergeAll,
	)
	return err
}

// UIDByEmail returns the Firebase UID for a given email address.
func UIDByEmail(ctx context.Context, email string) (string, error) {
	u, err := fbAuth.GetUserByEmail(ctx, email)
	if err != nil {
		return "", err
	}
	return u.UID, nil
}
