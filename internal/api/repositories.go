package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/harshpatel5940/gitvigil/internal/models"
)

type RepositoryResponse struct {
	ID             int64      `json:"id"`
	GitHubID       int64      `json:"github_id"`
	InstallationID int64      `json:"installation_id"`
	Owner          string     `json:"owner"`
	Name           string     `json:"name"`
	FullName       string     `json:"full_name"`
	HasLicense     bool       `json:"has_license"`
	LicenseSPDXID  *string    `json:"license_spdx_id,omitempty"`
	StreakStatus   string     `json:"streak_status"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	AlertsCount    int        `json:"alerts_count"`
	CommitsCount   int        `json:"commits_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type RepositoriesListResponse struct {
	Repositories []RepositoryResponse `json:"repositories"`
	Total        int                  `json:"total"`
	Page         int                  `json:"page"`
	PerPage      int                  `json:"per_page"`
}

func repoToResponse(r *models.RepositoryWithStats) RepositoryResponse {
	return RepositoryResponse{
		ID:             r.ID,
		GitHubID:       r.GitHubID,
		InstallationID: r.InstallationID,
		Owner:          r.Owner,
		Name:           r.Name,
		FullName:       r.FullName,
		HasLicense:     r.HasLicense,
		LicenseSPDXID:  r.LicenseSPDXID,
		StreakStatus:   r.StreakStatus,
		LastActivityAt: r.LastActivityAt,
		AlertsCount:    r.AlertsCount,
		CommitsCount:   r.CommitsCount,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

func (h *Handler) ListRepositories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	pagination := h.getPagination(r)

	store := models.NewRepositoryStore(h.db.Pool)
	repos, total, err := store.ListAll(ctx, pagination.PerPage, pagination.Offset)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list repositories")
		h.respondError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}

	response := RepositoriesListResponse{
		Repositories: make([]RepositoryResponse, 0, len(repos)),
		Total:        total,
		Page:         pagination.Page,
		PerPage:      pagination.PerPage,
	}

	for _, repo := range repos {
		response.Repositories = append(response.Repositories, repoToResponse(repo))
	}

	h.respondJSON(w, http.StatusOK, response)
}

func (h *Handler) GetRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid repository ID")
		return
	}

	store := models.NewRepositoryStore(h.db.Pool)
	repo, err := store.GetByID(ctx, id)
	if err != nil {
		h.logger.Error().Err(err).Int64("id", id).Msg("failed to get repository")
		h.respondError(w, http.StatusNotFound, "repository not found")
		return
	}

	h.respondJSON(w, http.StatusOK, repoToResponse(repo))
}
