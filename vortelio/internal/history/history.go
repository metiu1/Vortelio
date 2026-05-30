package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/vortelio/vortelio/internal/config"
)

// Entry è una singola conversazione salvata.
type Entry struct {
	ID        string    `json:"id"`
	Model     string    `json:"model"`
	Role      string    `json:"role"` // "user" | "assistant"
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

func histPath() string {
	return filepath.Join(config.HomeDir(), "history.ndjson")
}

// Append aggiunge una o più entry al file storia.
func Append(entries ...Entry) error {
	os.MkdirAll(filepath.Dir(histPath()), 0700)
	f, err := os.OpenFile(histPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if e.CreatedAt.IsZero() {
			e.CreatedAt = time.Now()
		}
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// List legge le ultime n entry. Se n <= 0, legge tutte.
func List(n int) ([]Entry, error) {
	f, err := os.Open(histPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 512*1024), 512*1024)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			all = append(all, e)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	if n > 0 && len(all) > n {
		all = all[len(all)-n:]
	}
	return all, nil
}

// Clear elimina tutta la storia.
func Clear() error {
	return os.Remove(histPath())
}
