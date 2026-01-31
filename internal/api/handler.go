package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/harshpatel5940/gitvigil/internal/database"
	"github.com/rs/zerolog"
)

type Handler struct {
	db     *database.DB
	logger zerolog.Logger
}

func NewHandler(db *database.DB, logger zerolog.Logger) *Handler {
	return &Handler{
		db:     db,
		logger: logger.With().Str("component", "api").Logger(),
	}
}

// Router returns a chi router with all API routes
func (h *Handler) Router() chi.Router {
	r := chi.NewRouter()

	// Repositories
	r.Get("/repositories", h.ListRepositories)
	r.Get("/repositories/{id}", h.GetRepository)

	// Installations
	r.Get("/installations", h.ListInstallations)
	r.Get("/installations/{id}", h.GetInstallation)
	r.Get("/installations/{id}/repositories", h.ListInstallationRepositories)

	// Stats
	r.Get("/stats", h.GetStats)

	return r
}

// JSON response helpers

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error().Err(err).Msg("failed to encode JSON response")
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}

// Pagination helpers

type PaginationParams struct {
	Page    int
	PerPage int
	Offset  int
}

func (h *Handler) getPagination(r *http.Request) PaginationParams {
	page := 1
	perPage := 20

	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}

	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
	}

	return PaginationParams{
		Page:    page,
		PerPage: perPage,
		Offset:  (page - 1) * perPage,
	}
}
