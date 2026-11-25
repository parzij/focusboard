package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/tasks"
)

type taskJSON struct {
	ID          string
	Title       string
	Description string
	Status      tasks.Status
	Position    int
	DueDate     *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func decodeTaskResponse[T any](t *testing.T, response *httptest.ResponseRecorder, status int) T {
	t.Helper()
	checkAuthResponse(t, response, status, "")
	var value T
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func readBoard(t *testing.T, response *httptest.ResponseRecorder) []taskJSON {
	t.Helper()
	board := decodeTaskResponse[struct{ Tasks []taskJSON }](t, response, 200)
	if board.Tasks == nil {
		t.Fatal("expected a JSON array")
	}
	positions := map[tasks.Status]int{}
	order := map[tasks.Status]int{tasks.NotStarted: 0, tasks.InProgress: 1, tasks.Finish: 2, tasks.Done: 3}
	last := -1
	for _, task := range board.Tasks {
		rank, ok := order[task.Status]
		if !ok || rank < last || task.Position != positions[task.Status] {
			t.Fatalf("non-canonical board at position %d in %s", task.Position, task.Status)
		}
		last = rank
		positions[task.Status]++
	}
	return board.Tasks
}

type taskRequest func(string, string, string) *httptest.ResponseRecorder

func taskClient(t *testing.T, api authAPI, access string) taskRequest {
	return func(method, path, body string) *httptest.ResponseRecorder {
		return api.call(t, method, "/tasks"+path, body, "", access, nil)
	}
}

func assertTaskOrder(t *testing.T, board []taskJSON, expected ...taskJSON) {
	t.Helper()
	if len(board) != len(expected) {
		t.Fatalf("board length = %d, want %d", len(board), len(expected))
	}
	for index, task := range expected {
		if board[index].ID != task.ID {
			t.Fatalf("task order differs at index %d", index)
		}
	}
}

func TestTasksPostgreSQL(t *testing.T) {
	pool, provider := testDatabase(t, 12)
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	api := testAPI(t, pool)

	t.Run("CRUD move and history", func(t *testing.T) {
		owner := api.register(t, "tasks@example.com")
		request := taskClient(t, api, owner.AccessToken)
		brd := func() []taskJSON { return readBoard(t, request("GET", "", "")) }
		assertTaskOrder(t, brd())
		create := func(body string) taskJSON {
			response := request("POST", "", body)
			task := decodeTaskResponse[taskJSON](t, response, 201)
			var keys map[string]json.RawMessage
			if err := json.Unmarshal(response.Body.Bytes(), &keys); err != nil || len(keys) != 8 ||
				keys["dueDate"] == nil || task.ID == "" || task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
				t.Fatal("invalid public task shape")
			}
			return task
		}
		done := create(`{"title":"Created closed","status":"done"}`)
		finish := create(`{"title":"Finished","status":"finish"}`)
		progress := create(`{"title":"In progress","status":"in_progress"}`)
		a := create(`{"title":"  A  "}`)
		b := create(`{"title":"B","status":"not_started","dueDate":"2000-02-29"}`)
		c := create(`{"title":"C"}`)
		if a.Title != "A" || a.Status != tasks.NotStarted || a.Position != 0 || a.Description != "" || a.DueDate != nil ||
			b.DueDate == nil || *b.DueDate != "2000-02-29" || b.Position != 1 {
			t.Fatal("unexpected create defaults or calendar date")
		}
		assertTaskOrder(t, brd(), a, b, c, progress, finish, done)
		if got := decodeTaskResponse[taskJSON](t, request("GET", "/"+a.ID, ""), 200); !reflect.DeepEqual(got, a) {
			t.Fatal("GET differs from create")
		}
		patch := func(task taskJSON, body string) taskJSON {
			return decodeTaskResponse[taskJSON](t, request("PATCH", "/"+task.ID, body), 200)
		}
		updated := patch(b, `{"title":" Beta ","description":"Description","status":"not_started","dueDate":null}`)
		if updated.Title != "Beta" || updated.Description != "Description" || updated.DueDate != nil || updated.Position != 1 {
			t.Fatal("patch did not preserve position or apply fields")
		}
		a = patch(a, `{"status":"in_progress","dueDate":"2026-11-30"}`)
		if a.Position != 1 || a.DueDate == nil || *a.DueDate != "2026-11-30" {
			t.Fatal("status patch did not append")
		}
		assertTaskOrder(t, brd(), b, c, progress, a, finish, done)

		move := func(task taskJSON, status tasks.Status, index int) []taskJSON {
			response := request("PATCH", "/"+task.ID+"/move", fmt.Sprintf(`{"status":%q,"position":%d}`, status, index))
			snapshot := readBoard(t, response)
			if !reflect.DeepEqual(snapshot, brd()) {
				t.Fatal("move response differs from committed board")
			}
			return snapshot
		}
		assertTaskOrder(t, move(b, tasks.NotStarted, 1), c, b, progress, a, finish, done)
		assertTaskOrder(t, move(b, tasks.NotStarted, 0), b, c, progress, a, finish, done)
		before := brd()
		if !reflect.DeepEqual(before, move(b, tasks.NotStarted, 0)) {
			t.Fatal("no-op move modified a task")
		}
		checkAuthResponse(t, request("PATCH", "/"+b.ID+"/move", `{"status":"not_started","position":2}`), 422, "VALIDATION_ERROR")
		assertTaskOrder(t, move(a, tasks.Done, 0), b, c, progress, finish, a, done)
		assertTaskOrder(t, move(done, tasks.Finish, 1), b, c, progress, finish, done, a)
		assertTaskOrder(t, move(done, tasks.Done, 1), b, c, progress, finish, a, done)
		assertTaskOrder(t, move(progress, tasks.NotStarted, 2), b, c, progress, finish, a, done)
		assertTaskOrder(t, move(finish, tasks.InProgress, 0), b, c, progress, finish, a, done)

		checkAuthResponse(t, request("DELETE", "/"+c.ID, ""), 204, "")
		assertTaskOrder(t, brd(), b, progress, finish, a, done)
		assertCount(t, pool, 1, "SELECT count(*) FROM tasks WHERE id=$1 AND deleted_at IS NOT NULL AND updated_at=deleted_at AND position IS NULL", c.ID)
		assertHistory(t, pool, b.ID, "->not_started")
		assertHistory(t, pool, c.ID, "->not_started")
		assertHistory(t, pool, a.ID, "->not_started", "not_started->in_progress", "in_progress->done")
		assertHistory(t, pool, done.ID, "->done", "done->finish", "finish->done")
	})

	t.Run("HTTP validation and owner isolation", func(t *testing.T) {
		a := api.register(t, "tasks-owner@example.com")
		b := api.register(t, "tasks-other@example.com")
		owner, other := taskClient(t, api, a.AccessToken), taskClient(t, api, b.AccessToken)
		task := decodeTaskResponse[taskJSON](t, owner("POST", "", `{"title":"Private"}`), 201)
		path := "/" + task.ID
		missing := owner("GET", "/00000000-0000-4000-8000-000000000000", "")
		assertTaskOrder(t, readBoard(t, other("GET", "?userId="+a.User.ID, "")))
		cfg := integrationConfig().Auth
		issued := time.Now().Add(-time.Hour)
		expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
			Subject: a.User.ID, Issuer: cfg.Issuer, Audience: jwt.ClaimStrings{cfg.Audience}, ID: uuid.NewString(),
			IssuedAt: jwt.NewNumericDate(issued), ExpiresAt: jwt.NewNumericDate(issued.Add(cfg.AccessTTL)),
		}).SignedString(cfg.Secret)
		if err != nil {
			t.Fatal(err)
		}
		for _, endpoint := range []struct{ method, path, payload string }{
			{"GET", "", ""}, {"POST", "", `{"title":"New"}`}, {"GET", path, ""},
			{"PATCH", path, `{"title":"Changed"}`}, {"PATCH", path + "/move", `{"status":"done","position":0}`},
			{"DELETE", path, ""},
		} {
			for _, access := range []string{"", expired, "invalid"} {
				checkAuthResponse(t, taskClient(t, api, access)(endpoint.method, endpoint.path, endpoint.payload), 401, "UNAUTHORIZED")
			}
			if endpoint.path != "" {
				foreign := other(endpoint.method, endpoint.path, endpoint.payload)
				checkAuthResponse(t, foreign, 404, "TASK_NOT_FOUND")
				if foreign.Body.String() != missing.Body.String() {
					t.Fatal("foreign task response differs from missing task")
				}
			}
		}
		for _, body := range []string{`null`, `[]`, `{`, `{} {}`, `{"title":7}`, `{"title":"x","title":"y"}`,
			`{"title":"x","userId":"` + a.User.ID + `"}`, `{"title":"x","createdAt":null}`,
			`{"title":"` + strings.Repeat("x", 64<<10) + `"}`} {
			checkAuthResponse(t, other("POST", "", body), 400, "VALIDATION_ERROR")
		}
		for _, body := range []string{`{}`, `{"title":null}`, `{"title":"  "}`, `{"description":null}`,
			`{"status":"submitted"}`, `{"dueDate":"2026-02-29"}`} {
			checkAuthResponse(t, owner("PATCH", path, body), 422, "VALIDATION_ERROR")
		}
		checkAuthResponse(t, owner("GET", "/not-a-uuid", ""), 422, "VALIDATION_ERROR")
		checkAuthResponse(t, owner("PATCH", path, `{"position":4}`), 400, "VALIDATION_ERROR")
		checkAuthResponse(t, owner("PATCH", path+"/move", `{"status":"done","position":"0"}`), 400, "VALIDATION_ERROR")
		for _, position := range []string{"-1", "0.5", "2147483648", "9223372036854775808", "null"} {
			checkAuthResponse(t, owner("PATCH", path+"/move", `{"status":"done","position":`+position+`}`), 422, "VALIDATION_ERROR")
		}
		checkAuthResponse(t, owner("DELETE", path, ""), 204, "")
		checkAuthResponse(t, owner("DELETE", path, ""), 404, "TASK_NOT_FOUND")
		checkAuthResponse(t, owner("GET", path, ""), 404, "TASK_NOT_FOUND")
		checkAuthResponse(t, owner("PATCH", path, `{"title":"Gone"}`), 404, "TASK_NOT_FOUND")
		checkAuthResponse(t, owner("PATCH", path+"/move", `{"status":"done","position":0}`), 404, "TASK_NOT_FOUND")
		assertTaskOrder(t, readBoard(t, owner("GET", "", "")))
	})

	t.Run("concurrent board mutations", func(t *testing.T) {
		a := api.register(t, "tasks-concurrent@example.com")
		b := api.register(t, "tasks-independent@example.com")
		first, second := taskClient(t, api, a.AccessToken), taskClient(t, api, b.AccessToken)
		var calls []func() *httptest.ResponseRecorder
		for i := range 8 {
			calls = append(calls, func() *httptest.ResponseRecorder {
				return first("POST", "", fmt.Sprintf(`{"title":"Card %d"}`, i))
			})
		}
		calls = append(calls, func() *httptest.ResponseRecorder { return second("POST", "", `{"title":"Other board"}`) })
		for _, result := range concurrentRequests(t, calls...) {
			checkAuthResponse(t, result, 201, "")
		}
		board := readBoard(t, first("GET", "", ""))
		if len(board) != 8 || len(readBoard(t, second("GET", "", ""))) != 1 {
			t.Fatal("concurrent creates lost tasks or mixed owners")
		}
		path := func(index int) string { return "/" + board[index].ID }
		move := func(index int) *httptest.ResponseRecorder {
			return first("PATCH", path(index)+"/move", `{"status":"done","position":0}`)
		}
		results := concurrentRequests(t,
			func() *httptest.ResponseRecorder { return move(0) },
			func() *httptest.ResponseRecorder { return move(1) },
			func() *httptest.ResponseRecorder { return first("PATCH", path(0), `{"title":"Edited"}`) },
			func() *httptest.ResponseRecorder { return first("PATCH", path(0), `{"description":"Preserved"}`) },
			func() *httptest.ResponseRecorder { return first("DELETE", path(2), "") },
			func() *httptest.ResponseRecorder { return move(2) },
		)
		for _, result := range results[:2] {
			if len(readBoard(t, result)) < 7 {
				t.Fatal("move returned a partial board")
			}
		}
		checkAuthResponse(t, results[2], 200, "")
		checkAuthResponse(t, results[3], 200, "")
		checkAuthResponse(t, results[4], 204, "")
		if results[5].Code != 200 {
			checkAuthResponse(t, results[5], 404, "TASK_NOT_FOUND")
			assertHistory(t, pool, board[2].ID, "->not_started")
		} else {
			assertHistory(t, pool, board[2].ID, "->not_started", "not_started->done")
		}
		updated := decodeTaskResponse[taskJSON](t, first("GET", path(0), ""), 200)
		if updated.Title != "Edited" || updated.Description != "Preserved" || updated.Status != tasks.Done {
			t.Fatal("concurrent PATCH or move lost changes")
		}
		if len(readBoard(t, first("GET", "", ""))) != 7 || len(readBoard(t, second("GET", "", ""))) != 1 {
			t.Fatal("unexpected final board sizes")
		}
		assertHistory(t, pool, board[0].ID, "->not_started", "not_started->done")
		assertHistory(t, pool, board[1].ID, "->not_started", "not_started->done")

		t.Run("another owner can write while the first owner is locked", func(t *testing.T) {
			tx, err := pool.Begin(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(t.Context())
			if _, err := tx.Exec(t.Context(), "SELECT id FROM users WHERE id=$1 FOR UPDATE", a.User.ID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			service := tasks.NewService(tasks.NewRepository(pool))
			title := "Independent write"
			created, err := service.Create(ctx, b.User.ID, tasks.Changes{Title: &title})
			if err != nil || created.Position != 1 {
				t.Fatalf("another owner's write was affected by the lock: %v", err)
			}
		})
	})

	t.Run("rollback", func(t *testing.T) {
		owner := api.register(t, "tasks-rollback@example.com")
		request := taskClient(t, api, owner.AccessToken)
		a := decodeTaskResponse[taskJSON](t, request("POST", "", `{"title":"A"}`), 201)
		b := decodeTaskResponse[taskJSON](t, request("POST", "", `{"title":"B"}`), 201)
		done := decodeTaskResponse[taskJSON](t, request("POST", "", `{"title":"Done","status":"done"}`), 201)
		snapshot := readBoard(t, request("GET", "", ""))
		assertUnchanged := func(t *testing.T) {
			t.Helper()
			if !reflect.DeepEqual(snapshot, readBoard(t, request("GET", "", ""))) {
				t.Fatal("failed operation changed the board")
			}
			assertHistory(t, pool, a.ID, "->not_started")
			assertHistory(t, pool, b.ID, "->not_started")
			assertHistory(t, pool, done.ID, "->done")
		}
		for _, fail := range []struct {
			name, table, event, body string
			status                   int
			code                     string
		}{
			{"position", "tasks", "UPDATE", `IF OLD.status=NEW.status AND OLD.position<>NEW.position THEN RAISE EXCEPTION 'test position failure'; END IF;`, 500, "INTERNAL_ERROR"},
			{"history", "task_history", "INSERT", `RAISE EXCEPTION 'test history failure';`, 500, "INTERNAL_ERROR"},
			{"deferred unique constraint", "tasks", "UPDATE", `IF NEW.status<>OLD.status THEN NEW.position=0; END IF;`, 409, "ORDER_CONFLICT"},
		} {
			t.Run(fail.name, func(t *testing.T) {
				execSQL(t, pool, "CREATE OR REPLACE FUNCTION reject_task_change() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN "+fail.body+" RETURN NEW; END; $$")
				execSQL(t, pool, "CREATE TRIGGER reject_task_change BEFORE "+fail.event+" ON "+fail.table+" FOR EACH ROW EXECUTE FUNCTION reject_task_change()")
				defer func() {
					if _, err := pool.Exec(t.Context(), "DROP TRIGGER reject_task_change ON "+fail.table); err != nil {
						t.Error(err)
					}
				}()
				checkAuthResponse(t, request("PATCH", "/"+a.ID+"/move", `{"status":"done","position":1}`), fail.status, fail.code)
				assertUnchanged(t)
			})
		}
		t.Run("detected gap in the destination column", func(t *testing.T) {
			execSQL(t, pool, "UPDATE tasks SET position=2 WHERE id=$1", done.ID)
			checkAuthResponse(t, request("PATCH", "/"+a.ID+"/move", `{"status":"done","position":0}`), 409, "ORDER_CONFLICT")
			execSQL(t, pool, "UPDATE tasks SET position=0 WHERE id=$1", done.ID)
			assertUnchanged(t)
		})
		t.Run("context timeout after position update", func(t *testing.T) {
			execSQL(t, pool, `CREATE FUNCTION slow_task_history() RETURNS trigger LANGUAGE plpgsql AS $$
				BEGIN PERFORM pg_sleep(5); RETURN NEW; END; $$`)
			execSQL(t, pool, `CREATE TRIGGER slow_task_history BEFORE INSERT ON task_history
				FOR EACH ROW EXECUTE FUNCTION slow_task_history()`)
			ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
			defer cancel()
			service := tasks.NewService(tasks.NewRepository(pool))
			index := int64(1)
			if _, err := service.Move(ctx, owner.User.ID, a.ID, tasks.Done, &index); !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected deadline, got %v", err)
			}
			assertUnchanged(t)
		})
	})
}

func assertHistory(t *testing.T, pool *pgxpool.Pool, id string, expected ...string) {
	t.Helper()
	var history []string
	err := pool.QueryRow(t.Context(), `SELECT array_agg(coalesce(old_status, '') || '->' || new_status
		ORDER BY changed_at, id) FROM task_history WHERE task_id=$1`, id).Scan(&history)
	if err != nil || !reflect.DeepEqual(history, expected) {
		t.Fatalf("history = %v, want %v (%v)", history, expected, err)
	}
}
