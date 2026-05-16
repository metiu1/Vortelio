package firebase

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MonthlyUsage tracks token consumption per user per billing month.
type MonthlyUsage struct {
	Month      string `firestore:"month"       json:"month"`        // "2026-05"
	TokensIn   int64  `firestore:"tokens_in"   json:"tokens_in"`
	TokensOut  int64  `firestore:"tokens_out"  json:"tokens_out"`
	Requests   int64  `firestore:"requests"    json:"requests"`
	LastUpdate time.Time `firestore:"last_update" json:"last_update"`
}

// Plan token limits per month (0 = unlimited).
var PlanLimits = map[string]int64{
	"free":       0,       // no cloud access
	"pro":        2000000, // 2M tokens/month
	"enterprise": 10000000,// 10M tokens/month
}

// currentMonth returns "YYYY-MM".
func currentMonth() string {
	return time.Now().UTC().Format("2006-01")
}

func usageDocRef(uid string) *firestore.DocumentRef {
	return fbStore.Collection("users").Doc(uid).
		Collection("usage").Doc(currentMonth())
}

// GetUsage returns token usage for the current month.
func GetUsage(ctx context.Context, uid string) (*MonthlyUsage, error) {
	snap, err := usageDocRef(uid).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return &MonthlyUsage{Month: currentMonth()}, nil
		}
		return nil, err
	}
	var u MonthlyUsage
	snap.DataTo(&u)
	return &u, nil
}

// CheckLimit returns an error if the user has exceeded their plan's monthly limit.
func CheckLimit(ctx context.Context, uid, plan string) error {
	limit, ok := PlanLimits[plan]
	if !ok || limit == 0 {
		if plan == "free" {
			return fmt.Errorf("cloud models require a Pro plan — upgrade at vortelio.app/upgrade")
		}
		return nil // unknown plan or unlimited
	}
	u, err := GetUsage(ctx, uid)
	if err != nil {
		return nil // don't block on usage read failure
	}
	total := u.TokensIn + u.TokensOut
	if total >= limit {
		return fmt.Errorf("monthly token limit reached (%d/%d) — resets on the 1st", total, limit)
	}
	return nil
}

// RecordUsage increments the monthly token counters atomically.
func RecordUsage(ctx context.Context, uid string, tokensIn, tokensOut int64) error {
	ref := usageDocRef(uid)
	return fbStore.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		snap, err := tx.Get(ref)
		var u MonthlyUsage
		if err != nil {
			if status.Code(err) != codes.NotFound {
				return err
			}
			u = MonthlyUsage{Month: currentMonth()}
		} else {
			snap.DataTo(&u)
		}
		u.TokensIn += tokensIn
		u.TokensOut += tokensOut
		u.Requests++
		u.LastUpdate = time.Now()
		return tx.Set(ref, u)
	})
}
