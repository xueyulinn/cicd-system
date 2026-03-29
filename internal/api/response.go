package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// WriteJSONError writes a JSON error response with the standard shape {"error": "message"}.
// All server-side handlers should use this for error responses so clients get a consistent format.
// Use statusCode for the HTTP status (e.g. http.StatusBadRequest, http.StatusInternalServerError).
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	WriteJSON(w, statusCode, map[string]string{"error": message})
}

func WriteJSON(w http.ResponseWriter, statusCode int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if _, err := w.Write(buf.Bytes()); err != nil {
		slog.Error("failed to write response", "error", err)
	}
}
