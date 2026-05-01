package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
)

// WriteJSONError writes a JSON error response with a default code derived from
// the HTTP status and the provided message.
func WriteJSONError(w http.ResponseWriter, statusCode int, message string) {
	WriteError(w, statusCode, defaultErrorCode(statusCode), message)
}

// WriteError writes a JSON error response with the standard shape
// {"code":"...", "message":"..."}.
func WriteError(w http.ResponseWriter, statusCode int, code string, message string) {
	WriteJSON(w, statusCode, &ErrorResponse{
		Code:    code,
		Message: message,
	})
}

// WriteJSON writes v as a JSON response with the provided HTTP status code.
// It sets Content-Type to application/json and falls back to http.Error if
// the payload cannot be encoded.
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

func defaultErrorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusBadGateway:
		return "bad_gateway"
	case http.StatusGatewayTimeout:
		return "gateway_timeout"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusInternalServerError:
		return "internal_error"
	default:
		return "error"
	}
}
