package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, err error) {
	status, code := httpStatusFromError(err)
	writeJSON(w, status, errorResponse{Code: code, Message: err.Error()})
}

func httpStatusFromError(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict, "ALREADY_EXISTS"
	case errors.Is(err, domain.ErrInvalidInput):
		return http.StatusBadRequest, "INVALID_INPUT"
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, "FORBIDDEN"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}
