package users

import (
	"errors"
	"net/http"
	"time"

	"github.com/parzij/focusboard/backend/internal/httpx"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.Profile(r.Context(), httpx.UserID(r.Context()))
	writeProfile(w, r, user, err)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	input, ok := httpx.ReadJSON[struct {
		Name string `json:"name"`
	}](w, r)
	if !ok {
		return
	}
	user, err := h.service.Rename(r.Context(), httpx.UserID(r.Context()), input.Name)
	writeProfile(w, r, user, err)
}

func writeProfile(w http.ResponseWriter, r *http.Request, user User, err error) {
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusOK, PublicProfile(user))
	case errors.Is(err, ErrInvalidName):
		httpx.ValidationError(w, map[string]string{"name": NameError})
	case errors.Is(err, ErrNotFound):
		httpx.Unauthorized(w)
	default:
		httpx.InternalError(w, r, "user profile", err)
	}
}

// PublicProfile is the JSON representation shared by the profile and session endpoints.
type PublicProfile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}
