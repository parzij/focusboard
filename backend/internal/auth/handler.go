package auth

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/parzij/focusboard/backend/internal/config"
	"github.com/parzij/focusboard/backend/internal/httpx"
	"github.com/parzij/focusboard/backend/internal/platform/password"
	"github.com/parzij/focusboard/backend/internal/users"
)

const cookiePath = "/api/v1/auth"

type Handler struct {
	service  *Service
	settings config.Auth
	limits   *limiter
}

func NewHandler(service *Service, settings config.Auth) *Handler {
	return &Handler{service: service, settings: settings, limits: newLimiter(settings.RateWindow, settings.RateKeys)}
}

func (h *Handler) Routes(router chi.Router) {
	router.Use(httpx.Deadline(h.settings.RequestTimeout), httpx.RequireOrigin(h.settings.Origin))
	router.With(h.limitIP).Post("/register", h.register)
	router.With(h.limitIP).Post("/login", h.login)
	router.With(h.limitIP).Post("/refresh", h.refresh)
	router.Post("/logout", h.logout)
}

type registerInput struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type passwordInput struct {
	Current string `json:"currentPassword"`
	New     string `json:"newPassword"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	body, ok := httpx.ReadJSON[registerInput](w, r)
	if !ok {
		return
	}
	email := normalizeEmail(body.Email)
	if email != "" && !h.allow(w, "account:"+email, h.settings.AccountLimit) {
		return
	}
	result, err := h.service.Register(r.Context(), body.Name, email, body.Password)
	h.respond(w, r, http.StatusCreated, result, err, "register")
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	body, ok := httpx.ReadJSON[loginInput](w, r)
	if !ok {
		return
	}
	email := normalizeEmail(body.Email)
	if email != "" && !h.allow(w, "account:"+email, h.settings.AccountLimit) {
		return
	}
	result, err := h.service.Login(r.Context(), email, body.Password)
	h.respond(w, r, http.StatusOK, result, err, "login")
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	if !httpx.EmptyBody(w, r) {
		return
	}
	result, err := h.service.Refresh(r.Context(), h.cookieValue(r))
	h.respond(w, r, http.StatusOK, result, err, "refresh")
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if !httpx.EmptyBody(w, r) {
		return
	}
	if err := h.service.Logout(r.Context(), h.cookieValue(r)); err != nil {
		h.writeError(w, r, "logout", err)
		return
	}
	h.clearCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	id := httpx.UserID(r.Context())
	if !h.allow(w, "ip:"+peerIP(r.RemoteAddr), h.settings.IPLimit) ||
		!h.allow(w, "password:"+id, h.settings.AccountLimit) {
		return
	}
	body, ok := httpx.ReadJSON[passwordInput](w, r)
	if !ok {
		return
	}
	if err := h.service.ChangePassword(r.Context(), id, body.Current, body.New); err != nil {
		if errors.Is(err, ErrCredentials) {
			httpx.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid current password.", nil)
			return
		}
		h.writeError(w, r, "change password", err)
		return
	}
	h.clearCookie(w)
	httpx.NoContent(w)
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, status int, result Result, err error, operation string) {
	if err != nil {
		h.writeError(w, r, operation, err)
		return
	}
	remaining := time.Until(result.RefreshUntil)
	seconds := int(remaining / time.Second)
	if remaining > 0 && seconds == 0 {
		seconds = 1
	}
	if seconds > 0 {
		h.setCookie(w, result.Refresh, result.RefreshUntil, seconds)
	} else {
		h.clearCookie(w)
	}
	httpx.WriteJSON(w, status, struct {
		AccessToken string              `json:"accessToken"`
		ExpiresIn   int                 `json:"expiresIn"`
		User        users.PublicProfile `json:"user"`
	}{AccessToken: result.AccessToken, ExpiresIn: result.ExpiresIn, User: users.PublicProfile(result.User)})
}

func (h *Handler) cookieValue(r *http.Request) string {
	cookies := r.CookiesNamed(h.settings.CookieName)
	if len(cookies) == 1 {
		return cookies[0].Value
	}
	return ""
}

func (h *Handler) clearCookie(w http.ResponseWriter) {
	h.setCookie(w, "", time.Unix(1, 0).UTC(), -1)
}

func (h *Handler) setCookie(w http.ResponseWriter, value string, expiry time.Time, age int) {
	http.SetCookie(w, &http.Cookie{Name: h.settings.CookieName, Path: cookiePath, Value: value,
		Expires: expiry.UTC(), MaxAge: age, HttpOnly: true, Secure: h.settings.CookieSecure, SameSite: http.SameSiteLaxMode})
}

func (h *Handler) limitIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.allow(w, "ip:"+peerIP(r.RemoteAddr), h.settings.IPLimit) {
			next.ServeHTTP(w, r)
		}
	})
}

func (h *Handler) allow(w http.ResponseWriter, key string, limit int) bool {
	allowed, retry := h.limits.allow(key, limit)
	if !allowed {
		rateLimited(w, retry)
	}
	return allowed
}

func rateLimited(w http.ResponseWriter, after time.Duration) {
	seconds := (after + time.Second - 1) / time.Second
	w.Header().Set("Retry-After", strconv.FormatInt(int64(seconds), 10))
	httpx.WriteError(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests. Try again later.", nil)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, operation string, err error) {
	var fields ValidationError
	switch {
	case errors.As(err, &fields):
		httpx.ValidationError(w, fields)
	case errors.Is(err, ErrCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid email or password.", nil)
	case errors.Is(err, ErrSession), errors.Is(err, users.ErrNotFound):
		httpx.Unauthorized(w)
	case errors.Is(err, ErrEmailExists):
		httpx.WriteError(w, http.StatusConflict, "EMAIL_ALREADY_EXISTS", "An account with this email already exists.", nil)
	case errors.Is(err, password.ErrBusy):
		rateLimited(w, time.Second)
	default:
		httpx.InternalError(w, r, operation, err)
	}
}
