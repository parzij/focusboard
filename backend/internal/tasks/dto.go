package tasks

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"time"
)

type field[T any] struct {
	present bool
	value   *T
}

func (f *field[T]) UnmarshalJSON(data []byte) error {
	f.present = true
	return json.Unmarshal(data, &f.value)
}

type taskInput struct {
	Title       field[string] `json:"title"`
	Description field[string] `json:"description"`
	Status      field[Status] `json:"status"`
	DueDate     field[string] `json:"dueDate"`
}

func (input taskInput) changes() (Changes, error) {
	fields := make(InvalidFields)
	for name, isNull := range map[string]bool{
		"title":       input.Title.present && input.Title.value == nil,
		"description": input.Description.present && input.Description.value == nil,
		"status":      input.Status.present && input.Status.value == nil,
	} {
		if isNull {
			fields[name] = "This field cannot be null."
		}
	}
	if len(fields) != 0 {
		return Changes{}, fields
	}
	return Changes{Title: input.Title.value, Description: input.Description.value, Status: input.Status.value,
		DueDate: input.DueDate.value, ClearDate: input.DueDate.present && input.DueDate.value == nil}, nil
}

type taskResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	Position    int       `json:"position"`
	DueDate     *string   `json:"dueDate"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func public(task Task) taskResponse {
	var due *string
	if task.DueDate != nil {
		value := task.DueDate.Format(time.DateOnly)
		due = &value
	}
	return taskResponse{task.ID, task.Title, task.Description, task.Status, task.Position, due,
		task.CreatedAt.UTC(), task.UpdatedAt.UTC()}
}

type moveInput struct {
	Status   Status        `json:"status"`
	Position positionInput `json:"position"`
}

// Invalid numeric values are field errors. Non-numeric JSON types are decoding errors.
type positionInput struct {
	value *int64
}

func (p *positionInput) UnmarshalJSON(data []byte) error {
	var value *int64
	err := json.Unmarshal(data, &value)
	if err != nil && jsontext.Value(data).Kind() == '0' {
		return nil
	}
	p.value = value
	return err
}
