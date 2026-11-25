package tasks

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/parzij/focusboard/backend/internal/httpx"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Routes(r chi.Router) {
	r.Get("/", h.list)
	r.Post("/", h.create)
	r.Get("/{id}", h.get)
	r.Patch("/{id}", h.update)
	r.Patch("/{id}/move", h.move)
	r.Delete("/{id}", h.delete)
}

func writeBoard(w http.ResponseWriter, r *http.Request, tasks []Task, err error) {
	if writeError(w, r, err) {
		return
	}
	result := make([]taskResponse, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, public(task))
	}
	httpx.WriteJSON(w, http.StatusOK, struct {
		Tasks []taskResponse `json:"tasks"`
	}{Tasks: result})
}

func writeError(w http.ResponseWriter, r *http.Request, err error) bool {
	var fields InvalidFields
	switch {
	case err == nil:
		return false
	case errors.As(err, &fields):
		httpx.ValidationError(w, fields)
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "TASK_NOT_FOUND", "Task not found.", nil)
	case errors.Is(err, ErrOwnerNotFound):
		httpx.Unauthorized(w)
	case errors.Is(err, ErrOrderConflict):
		httpx.WriteError(w, http.StatusConflict, "ORDER_CONFLICT", "Task order changed. Reload the board and try again.", nil)
	default:
		httpx.InternalError(w, r, "task request", err)
	}
	return true
}

func taskID(w http.ResponseWriter, r *http.Request) (string, bool) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil || len(raw) != 36 {
		httpx.ValidationError(w, map[string]string{"id": "Task ID must be a UUID."})
		return "", false
	}
	return id.String(), true
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.service.Board(r.Context(), httpx.UserID(r.Context()))
	writeBoard(w, r, tasks, err)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	if id, ok := taskID(w, r); ok {
		task, err := h.service.Find(r.Context(), httpx.UserID(r.Context()), id)
		if !writeError(w, r, err) {
			httpx.WriteJSON(w, http.StatusOK, public(task))
		}
	}
}

func readChanges(w http.ResponseWriter, r *http.Request) (Changes, bool) {
	input, ok := httpx.ReadJSON[taskInput](w, r)
	if !ok {
		return Changes{}, false
	}
	changes, err := input.changes()
	return changes, !writeError(w, r, err)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	if changes, ok := readChanges(w, r); ok {
		task, err := h.service.Create(r.Context(), httpx.UserID(r.Context()), changes)
		if !writeError(w, r, err) {
			httpx.WriteJSON(w, http.StatusCreated, public(task))
		}
	}
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if changes, ok := readChanges(w, r); ok {
		task, err := h.service.Update(r.Context(), httpx.UserID(r.Context()), id, changes)
		if !writeError(w, r, err) {
			httpx.WriteJSON(w, http.StatusOK, public(task))
		}
	}
}

func (h *Handler) move(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	input, ok := httpx.ReadJSON[moveInput](w, r)
	if !ok {
		return
	}
	tasks, err := h.service.Move(r.Context(), httpx.UserID(r.Context()), id, input.Status, input.Position.value)
	writeBoard(w, r, tasks, err)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if id, ok := taskID(w, r); ok {
		if !writeError(w, r, h.service.Delete(r.Context(), httpx.UserID(r.Context()), id)) {
			httpx.NoContent(w)
		}
	}
}
