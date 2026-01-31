package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/harshpatel5940/gitvigil/internal/models"
)

type InstallationResponse struct {
	ID             int64     `json:"id"`
	InstallationID int64     `json:"installation_id"`
	AccountLogin   string    `json:"account_login"`
	AccountType    string    `json:"account_type"`
	RepoCount      int       `json:"repo_count"`
	AlertCount     int       `json:"alert_count"`
	CommitCount    int       `json:"commit_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type InstallationsListResponse struct {
	Installations []InstallationResponse `json:"installations"`
	Total         int                    `json:"total"`
}

func installationToResponse(i *models.InstallationWithStats) InstallationResponse {
	return InstallationResponse{
		ID:             i.ID,
		InstallationID: i.InstallationID,
		AccountLogin:   i.AccountLogin,
		AccountType:    i.AccountType,
		RepoCount:      i.RepoCount,
		AlertCount:     i.AlertCount,
		CommitCount:    i.CommitCount,
		CreatedAt:      i.CreatedAt,
		UpdatedAt:      i.UpdatedAt,
	}
}

func (h *Handler) ListInstallations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	store := models.NewInstallationStore(h.db.Pool)
	installations, err := store.List(ctx)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to list installations")
		h.respondError(w, http.StatusInternalServerError, "failed to list installations")
		return
	}

	response := InstallationsListResponse{
		Installations: make([]InstallationResponse, 0, len(installations)),
		Total:         len(installations),
	}

	for _, inst := range installations {
		response.Installations = append(response.Installations, installationToResponse(inst))
	}

	h.respondJSON(w, http.StatusOK, response)
}

func (h *Handler) GetInstallation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid installation ID")
		return
	}

	store := models.NewInstallationStore(h.db.Pool)
	installation, err := store.GetByID(ctx, id)
	if err != nil {
		h.logger.Error().Err(err).Int64("id", id).Msg("failed to get installation")
		h.respondError(w, http.StatusNotFound, "installation not found")
		return
	}

	h.respondJSON(w, http.StatusOK, installationToResponse(installation))
}

func (h *Handler) ListInstallationRepositories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid installation ID")
		return
	}

	store := models.NewRepositoryStore(h.db.Pool)
	repos, err := store.ListByInstallation(ctx, id)
	if err != nil {
		h.logger.Error().Err(err).Int64("installation_id", id).Msg("failed to list repositories")
		h.respondError(w, http.StatusInternalServerError, "failed to list repositories")
		return
	}

	// Convert to response format (simple version without stats for this endpoint)
	type SimpleRepo struct {
		ID             int64      `json:"id"`
		GitHubID       int64      `json:"github_id"`
		FullName       string     `json:"full_name"`
		HasLicense     bool       `json:"has_license"`
		StreakStatus   string     `json:"streak_status"`
		LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	}

	response := struct {
		Repositories []SimpleRepo `json:"repositories"`
		Total        int          `json:"total"`
	}{
		Repositories: make([]SimpleRepo, 0, len(repos)),
		Total:        len(repos),
	}

	for _, repo := range repos {
		response.Repositories = append(response.Repositories, SimpleRepo{
			ID:             repo.ID,
			GitHubID:       repo.GitHubID,
			FullName:       repo.FullName,
			HasLicense:     repo.HasLicense,
			StreakStatus:   repo.StreakStatus,
			LastActivityAt: repo.LastActivityAt,
		})
	}

	h.respondJSON(w, http.StatusOK, response)
}
