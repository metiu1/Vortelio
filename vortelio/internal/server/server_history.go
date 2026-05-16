package server

import (
	"net/http"
	"strconv"

	"github.com/vortelio/vortelio/internal/history"
)

func handleHistory(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		nStr := r.URL.Query().Get("n")
		n, _ := strconv.Atoi(nStr)
		entries, err := history.List(n)
		if err != nil { jsonError(w, 500, err.Error()); return }
		if entries == nil { entries = []history.Entry{} }
		respond(w, 200, map[string]interface{}{"history": entries, "count": len(entries)})
	case http.MethodDelete:
		if err := history.Clear(); err != nil { jsonError(w, 500, err.Error()); return }
		respond(w, 200, map[string]string{"status": "cleared"})
	default:
		jsonError(w, 405, "use GET or DELETE")
	}
}
