package api

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSONError writes a JSON error response with the standard shape {"error": "message"}.
// All server-side handlers should use this for error responses so clients get a consistent format.
// Use statusCode for the HTTP status (e.g. http.StatusBadRequest, http.StatusInternalServerError).
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("[api] failed to encode error response: %v", err)
	}
}
