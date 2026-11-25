package tasks

import (
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

type Status string

const (
	NotStarted Status = "not_started"
	InProgress Status = "in_progress"
	Finish     Status = "finish"
	Done       Status = "done"
)

func (s Status) valid() bool {
	switch s {
	case NotStarted, InProgress, Finish, Done:
		return true
	default:
		return false
	}
}

type Task struct {
	ID          string
	Title       string
	Description string
	Status      Status
	Position    int
	DueDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Changes struct {
	Title       *string
	Description *string
	Status      *Status
	DueDate     *string
	ClearDate   bool
}

var (
	ErrNotFound      = errors.New("task not found")
	ErrOrderConflict = errors.New("task order conflict")
	ErrOwnerNotFound = errors.New("task owner not found")
)

type InvalidFields map[string]string

func (InvalidFields) Error() string { return "invalid task fields" }

func (c *Changes) validate() (*time.Time, error) {
	fields := make(InvalidFields)
	if c.Title != nil {
		title := strings.TrimSpace(*c.Title)
		if count := utf8.RuneCountInString(title); count < 1 || count > 120 || strings.ContainsRune(title, 0) {
			fields["title"] = "Title must contain 1–120 characters."
		}
		c.Title = &title
	}
	if c.Description != nil && (utf8.RuneCountInString(*c.Description) > 2000 || strings.ContainsRune(*c.Description, 0)) {
		fields["description"] = "Description must contain at most 2000 characters."
	}
	if c.Status != nil && !c.Status.valid() {
		fields["status"] = "Choose a valid status."
	}
	var due *time.Time
	if c.DueDate != nil {
		date, err := time.Parse(time.DateOnly, *c.DueDate)
		if err != nil || date.Year() < 1 || date.Format(time.DateOnly) != *c.DueDate {
			fields["dueDate"] = "Enter a valid date in YYYY-MM-DD format."
		} else {
			due = &date
		}
	}
	if len(fields) != 0 {
		return nil, fields
	}
	return due, nil
}

func validateMove(status Status, position *int64) error {
	fields := make(InvalidFields)
	if !status.valid() {
		fields["status"] = "Choose a valid status."
	}
	if position == nil || *position < 0 || *position > math.MaxInt32 {
		fields["position"] = "Position must be a non-negative integer in range."
	}
	if len(fields) != 0 {
		return fields
	}
	return nil
}
