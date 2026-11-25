package tasks

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestFields(t *testing.T) {
	for _, test := range []struct {
		name string
		edit Changes
		bad  string
	}{
		{"trim and Unicode maximum", Changes{Title: ptr("　" + strings.Repeat("界", 120) + "\t")}, ""},
		{"blank", Changes{Title: ptr("　 \n")}, "title"},
		{"title too long", Changes{Title: ptr(strings.Repeat("界", 121))}, "title"},
		{"description at maximum", Changes{Description: ptr(strings.Repeat("🙂", 2000))}, ""},
		{"description too long", Changes{Description: ptr(strings.Repeat("🙂", 2001))}, "description"},
		{"NUL rejected", Changes{Description: ptr("a\x00b")}, "description"},
		{"invalid status", Changes{Status: ptr(Status("completed"))}, "status"},
		{"past leap date", Changes{DueDate: ptr("2000-02-29")}, ""},
		{"invalid leap date", Changes{DueDate: ptr("1900-02-29")}, "dueDate"},
		{"year zero", Changes{DueDate: ptr("0000-01-01")}, "dueDate"},
		{"invalid month", Changes{DueDate: ptr("2026-13-01")}, "dueDate"},
		{"timestamp", Changes{DueDate: ptr("2026-11-30T00:00:00Z")}, "dueDate"},
		{"not padded", Changes{DueDate: ptr("2026-11-1")}, "dueDate"},
	} {
		t.Run(test.name, func(t *testing.T) {
			date, err := test.edit.validate()
			var fields InvalidFields
			if test.bad == "" {
				if err != nil {
					t.Fatal(err)
				}
				if date != nil && date.Format(time.DateOnly) != *test.edit.DueDate {
					t.Fatal("calendar date changed")
				}
				if test.edit.Title != nil && *test.edit.Title != strings.Repeat("界", 120) {
					t.Fatal("title was not trimmed")
				}
			} else if !errors.As(err, &fields) || fields[test.bad] == "" {
				t.Fatalf("expected error for %s, got %v", test.bad, err)
			}
		})
	}
	for _, status := range []Status{NotStarted, InProgress, Finish, Done} {
		if err := validateMove(status, ptr(int64(0))); err != nil {
			t.Fatal(err)
		}
	}
	for _, position := range []*int64{nil, ptr(int64(-1)), ptr(int64(maxPosition) + 1)} {
		if err := validateMove(Done, position); err == nil {
			t.Fatal("invalid position accepted")
		}
	}
}

func TestPatchPresence(t *testing.T) {
	for _, test := range []struct {
		input string
		clear bool
		value bool
		bad   bool
	}{
		{`{}`, false, false, false},
		{`{"dueDate":null}`, true, false, false},
		{`{"dueDate":"2026-11-30"}`, false, true, false},
		{`{"title":null}`, false, false, true},
		{`{"description":null}`, false, false, true},
		{`{"status":null}`, false, false, true},
	} {
		t.Run(test.input, func(t *testing.T) {
			var input taskInput
			if err := json.Unmarshal([]byte(test.input), &input); err != nil {
				t.Fatal(err)
			}
			change, err := input.changes()
			if (err != nil) != test.bad || change.ClearDate != test.clear || (change.DueDate != nil) != test.value {
				t.Fatalf("changes = %+v, err = %v", change, err)
			}
		})
	}
}

func TestMoveShifts(t *testing.T) {
	for _, test := range []struct {
		name                  string
		same                  bool
		current, target, size int
		source, destination   positionShift
		bad                   bool
	}{
		{"empty", false, 0, 0, 0, positionShift{1, maxPosition, -1}, positionShift{0, -1, 1}, false},
		{"same column to end", true, 0, 2, 3, positionShift{1, 2, -1}, positionShift{}, false},
		{"same column to start", true, 2, 0, 3, positionShift{0, 1, 1}, positionShift{}, false},
		{"no-op", true, 1, 1, 3, positionShift{}, positionShift{}, false},
		{"cross column to start", false, 1, 0, 3, positionShift{2, maxPosition, -1}, positionShift{0, 2, 1}, false},
		{"cross column to end", false, 1, 3, 3, positionShift{2, maxPosition, -1}, positionShift{3, 2, 1}, false},
		{"past the end", true, 0, 3, 3, positionShift{}, positionShift{}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			source, destination, err := moveShifts(test.same, test.current, test.target, test.size)
			if (err != nil) != test.bad || source != test.source || destination != test.destination {
				t.Fatalf("shifts = %+v, %+v, err = %v", source, destination, err)
			}
		})
	}
}
