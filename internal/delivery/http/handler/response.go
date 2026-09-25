package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/tnnz20/youthpreneur-be/internal/model"
)

// maxBodyBytes caps decoded JSON request bodies at 1 MiB to bound memory use.
const maxBodyBytes = 1 << 20

// decodeJSON reads one JSON object from the request body, writing a 400 and
// returning false when decoding fails or the body exceeds maxBodyBytes.
func decodeJSON(logger *slog.Logger, w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		if logger != nil {
			logger.Debug("failed to decode request body", "error", err)
		}
		WriteError(logger, w, http.StatusBadRequest, "invalid request body")
		return false
	}

	return true
}

// writeJSON encodes payload with Content-Type application/json and status.
func writeJSON(logger *slog.Logger, w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil && logger != nil {
		logger.Error("encoding response", "error", err)
	}
}

// WriteError encodes the standard error body. It is exported so middleware can
// share the same response shape without duplicating it.
func WriteError(logger *slog.Logger, w http.ResponseWriter, status int, message string) {
	if status < http.StatusInternalServerError && logger != nil {
		logger.Debug("client error response", "status", status, "message", message)
	}
	writeJSON(logger, w, status, model.ErrorResponse{Error: message})
}
