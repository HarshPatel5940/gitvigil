package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/harshpatel5940/gitvigil/internal/config"
	"github.com/rs/zerolog"
)

type Handler struct {
	cfg    *config.Config
	logger zerolog.Logger
}

func NewHandler(cfg *config.Config, logger zerolog.Logger) *Handler {
	return &Handler{
		cfg:    cfg,
		logger: logger.With().Str("component", "auth").Logger(),
	}
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
}

type UserInfo struct {
	Login     string `json:"login"`
	ID        int64  `json:"id"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`
	Email     string `json:"email"`
}

type CallbackResponse struct {
	Success bool      `json:"success"`
	User    *UserInfo `json:"user,omitempty"`
	Error   string    `json:"error,omitempty"`
}

func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		h.respondError(w, "missing code parameter", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	token, err := h.exchangeCodeForToken(code)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to exchange code for token")
		h.respondError(w, "failed to authenticate", http.StatusInternalServerError)
		return
	}

	// Get user info
	user, err := h.getUserInfo(token.AccessToken)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to get user info")
		h.respondError(w, "failed to get user info", http.StatusInternalServerError)
		return
	}

	h.logger.Info().
		Str("login", user.Login).
		Int64("id", user.ID).
		Msg("user authenticated")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(CallbackResponse{
		Success: true,
		User:    user,
	})
}

func (h *Handler) exchangeCodeForToken(code string) (*TokenResponse, error) {
	data := url.Values{}
	data.Set("client_id", h.cfg.ClientID)
	data.Set("client_secret", h.cfg.ClientSecret)
	data.Set("code", code)

	req, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed: %s", string(body))
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, err
	}

	return &token, nil
}

func (h *Handler) getUserInfo(accessToken string) (*UserInfo, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user info request failed: %s", string(body))
	}

	var user UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (h *Handler) respondError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(CallbackResponse{
		Success: false,
		Error:   message,
	})
}
