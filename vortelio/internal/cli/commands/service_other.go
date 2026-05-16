//go:build !windows

package commands

import (
	"fmt"
	"net/http"
	"time"
)

func IsServiceRunning(port string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://localhost:" + port + "/api/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func LaunchServiceDetached(port string) error { return nil }

func EnsureServiceRunning(port string) (bool, error) {
	if IsServiceRunning(port) {
		return true, nil
	}
	fmt.Printf("🚀  Avvio servizio Vortelio in background (porta %s)...\n", port)
	return false, fmt.Errorf("usa 'vortelio serve' in un terminale separato")
}
