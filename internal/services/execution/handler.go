package execution

import (
	"encoding/json"
	"io"
	"net/http"
)

type Handler struct {
	client *Client
}

func NewHandler() *Handler {
	return &Handler{
		client: NewClient(),
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/run", h.handleExecution)
}


func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}

}

func (h *Handler) handleExecution(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close() // Ignore close error as we're done with the body
	}()

	// Parse request
	var req RunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	} 

	// Run pipeline through execution service
	resp, err := h.client.Run(req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		if encErr := json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}); encErr != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if resp.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusBadRequest)
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}


// RunRequest is the input for running a pipeline.
type RunRequest struct {
	YAMLContent   string `json:"yaml_content"`
	Branch        string `json:"branch"`
	Commit        string `json:"commit"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}

