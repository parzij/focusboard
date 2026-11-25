package integration

import (
	"fmt"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/parzij/focusboard/backend/internal/app"
	"github.com/parzij/focusboard/backend/internal/platform/token"
	"github.com/parzij/focusboard/backend/internal/statistics"
)

type statisticsJSON struct {
	Period   string
	Timezone string
	Buckets  []struct {
		StartDate string
		Count     int64
	}
	ThisWeek  int64
	ThisMonth int64
}

func TestStatisticsPostgreSQL(t *testing.T) {
	pool, provider := testDatabase(t, 10)
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatal(err)
	}
	api := testAPI(t, pool)

	t.Run("first Done survives edits reopen and soft delete", func(t *testing.T) {
		owner := api.register(t, "stats-history@example.com")
		other := api.register(t, "stats-other@example.com")
		request := taskClient(t, api, owner.AccessToken)
		create := func(status string) taskJSON {
			return decodeTaskResponse[taskJSON](t, request("POST", "", fmt.Sprintf(`{"title":"Card","status":%q}`, status)), 201)
		}
		patch := func(task taskJSON, status string) {
			checkAuthResponse(t, request("PATCH", "/"+task.ID, fmt.Sprintf(`{"status":%q}`, status)), 200, "")
		}
		now := utc(t, "2026-11-25T12:00:00Z")
		report := func(id string, period statistics.Period) statistics.Report {
			return getStatistics(t, pool, id, now, period, "UTC")
		}
		assertStatistics(t, report(owner.User.ID, statistics.Days), 0, 0, 0, 0, 0, 0, 0, 0, 0)
		create("not_started")
		create("in_progress")
		create("finish")
		unclosed := create("finish")
		checkAuthResponse(t, request("DELETE", "/"+unclosed.ID, ""), 204, "")

		initial := create("done")
		dateHistory(t, pool, initial.ID, "2026-11-23T00:00:00Z")
		direct := create("not_started")
		patch(direct, "done")
		patch(direct, "in_progress")
		patch(direct, "done")
		dateHistory(t, pool, direct.ID, "2026-11-22T01:00:00Z", "2026-11-23T23:59:59.999999Z",
			"2026-11-24T00:00:00Z", "2026-11-25T10:00:00Z")

		moved := create("finish")
		checkAuthResponse(t, request("PATCH", "/"+moved.ID+"/move", `{"status":"done","position":0}`), 200, "")
		checkAuthResponse(t, request("PATCH", "/"+moved.ID+"/move", `{"status":"done","position":2}`), 200, "")
		checkAuthResponse(t, request("PATCH", "/"+moved.ID, `{"title":"Edited","dueDate":"2027-01-01"}`), 200, "")
		checkAuthResponse(t, request("DELETE", "/"+moved.ID, ""), 204, "")
		dateHistory(t, pool, moved.ID, "2026-11-23T00:00:00Z", "2026-11-24T00:00:00Z")

		reopened := create("done")
		patch(reopened, "finish")
		dateHistory(t, pool, reopened.ID, "2026-11-22T23:59:59.999999Z", "2026-11-23T00:00:00Z")

		priorMonth := create("done")
		patch(priorMonth, "finish")
		patch(priorMonth, "done")
		dateHistory(t, pool, priorMonth.ID, "2026-10-31T23:59:59.999999Z", "2026-11-01T00:00:00Z", "2026-11-25T06:00:00Z")

		priorWeek := create("done")
		patch(priorWeek, "not_started")
		patch(priorWeek, "done")
		dateHistory(t, pool, priorWeek.ID, "2026-11-01T00:00:00Z", "2026-11-17T00:00:00Z", "2026-11-25T06:00:00Z")

		tooOld := create("done")
		patch(tooOld, "finish")
		patch(tooOld, "done")
		dateHistory(t, pool, tooOld.ID, "2026-10-04T23:59:59.999999Z", "2026-11-23T09:00:00Z", "2026-11-25T10:00:00Z")

		addClosures(t, pool, owner.User.ID, "2026-11-25T12:00:00Z", "2026-11-25T12:00:00.000001Z", "2026-11-26T00:00:00Z")
		addClosures(t, pool, other.User.ID, "2026-11-25T11:00:00Z", "2026-11-25T11:01:00Z")

		assertStatistics(t, report(owner.User.ID, statistics.Days), 4, 6, 2, 1, 1, 0, 0, 0, 0)
		assertStatistics(t, report(owner.User.ID, statistics.Weeks), 4, 6, 0, 0, 0, 2, 0, 0, 1, 4)
		assertStatistics(t, report(other.User.ID, statistics.Days), 2, 2, 0, 0, 2, 0, 0, 0, 0)
	})

	t.Run("six active tasks and five first completions", func(t *testing.T) {
		owner := api.register(t, "stats-six@example.com")
		request := taskClient(t, api, owner.AccessToken)
		create := func(status string) taskJSON {
			return decodeTaskResponse[taskJSON](t, request("POST", "", fmt.Sprintf(`{"title":"Card","status":%q}`, status)), 201)
		}
		const first = "2026-11-25T09:00:00Z"
		now := utc(t, "2026-11-25T12:00:00Z")
		for range 5 {
			task := create("done")
			dateHistory(t, pool, task.ID, first)
		}
		pending := create("finish")
		dateHistory(t, pool, pending.ID, first)
		check := func(active int, completed int64) {
			t.Helper()
			if board := readBoard(t, request("GET", "", "")); len(board) != active {
				t.Fatalf("active tasks = %d, want %d", len(board), active)
			}
			assertStatistics(t, getStatistics(t, pool, owner.User.ID, now, statistics.Days, "UTC"),
				completed, completed, 0, 0, completed, 0, 0, 0, 0)
			assertStatistics(t, getStatistics(t, pool, owner.User.ID, now, statistics.Weeks, "UTC"),
				completed, completed, 0, 0, 0, 0, 0, 0, 0, completed)
		}
		check(6, 5)
		checkAuthResponse(t, request("PATCH", "/"+pending.ID+"/move", `{"status":"done","position":5}`), 200, "")
		dateHistory(t, pool, pending.ID, first, "2026-11-25T10:00:00Z")
		check(6, 6)
		checkAuthResponse(t, request("PATCH", "/"+pending.ID, `{"status":"finish"}`), 200, "")
		checkAuthResponse(t, request("PATCH", "/"+pending.ID, `{"status":"done"}`), 200, "")
		dateHistory(t, pool, pending.ID, first, "2026-11-25T10:00:00Z", "2026-11-25T10:30:00Z", "2026-11-25T11:00:00Z")
		check(6, 6)
		checkAuthResponse(t, request("DELETE", "/"+pending.ID, ""), 204, "")
		check(5, 6)
	})

	t.Run("calendar boundaries and DST", func(t *testing.T) {
		for i, test := range []struct {
			name, instant, timezone, monday string
			events                          []string
			week, month                     int64
			counts                          []int64
		}{
			{"spring forward", "2026-03-29T22:00:00Z", "Europe/Berlin", "2026-03-30",
				[]string{"2026-03-28T23:00:00Z", "2026-03-29T00:59:59.999999Z", "2026-03-29T01:00:00Z",
					"2026-03-29T21:59:59.999999Z", "2026-03-29T22:00:00Z"}, 1, 5, []int64{1, 0, 0, 0, 0, 0, 0}},
			{"fall back", "2026-11-01T23:00:00Z", "America/New_York", "2026-10-26",
				[]string{"2026-11-01T03:59:59.999999Z", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z",
					"2026-11-01T06:30:00Z", "2026-11-02T04:59:59.999999Z"}, 4, 3, []int64{0, 0, 0, 0, 0, 1, 3}},
			{"year boundary and half-hour offset", "2027-01-01T00:00:00Z", "Asia/Kolkata", "2026-12-28",
				[]string{"2026-12-31T18:29:59.999999Z", "2026-12-31T18:30:00Z"}, 2, 1, []int64{0, 0, 0, 1, 1, 0, 0}},
			{"same instants in UTC", "2027-01-01T00:00:00Z", "UTC", "2026-12-28",
				[]string{"2026-12-31T18:29:59.999999Z", "2026-12-31T18:30:00Z"}, 2, 0, []int64{0, 0, 0, 2, 0, 0, 0}},
			{"leap day and March", "2024-03-01T00:00:00Z", "UTC", "2024-02-26",
				[]string{"2024-02-29T00:00:00Z", "2024-02-29T23:59:59.999999Z", "2024-03-01T00:00:00Z"},
				3, 1, []int64{0, 0, 0, 2, 1, 0, 0}},
		} {
			t.Run(test.name, func(t *testing.T) {
				owner := api.register(t, fmt.Sprintf("stats-calendar-%d@example.com", i))
				addClosures(t, pool, owner.User.ID, test.events...)
				now := utc(t, test.instant)
				daily := getStatistics(t, pool, owner.User.ID, now, statistics.Days, test.timezone)
				assertStatistics(t, daily, test.week, test.month, test.counts...)
				if daily.Buckets[0].StartDate.Format(time.DateOnly) != test.monday {
					t.Fatal("incorrect first bucket date")
				}
				weekly := getStatistics(t, pool, owner.User.ID, now, statistics.Weeks, test.timezone)
				if weekly.ThisWeek != test.week || weekly.ThisMonth != test.month || weekly.Buckets[7].Count != test.week {
					t.Fatalf("inconsistent weekly report: %+v", weekly)
				}
				if i == 0 {
					if weekly.Buckets[6].Count != 4 {
						t.Fatalf("spring-forward Sunday has %d completions, want 4", weekly.Buckets[6].Count)
					}
					beforeMonday := getStatistics(t, pool, owner.User.ID, now.Add(-time.Microsecond), statistics.Days, test.timezone)
					assertStatistics(t, beforeMonday, 4, 4, 0, 0, 0, 0, 0, 0, 4)
				}
			})
		}
	})

	t.Run("single snapshot during concurrent writes", func(t *testing.T) {
		owner := api.register(t, "stats-concurrent@example.com")
		now := utc(t, "2026-11-25T12:00:00Z")
		service := statistics.NewService(statistics.NewRepository(pool), func() time.Time { return now })
		start := make(chan struct{})
		var group sync.WaitGroup
		results := make(chan error, 20)
		group.Go(func() {
			<-start
			for range cap(results) {
				_, err := pool.Exec(t.Context(), insertClosureSQL, owner.User.ID, now)
				results <- err
			}
		})
		close(start)
		defer group.Wait()
		for range 40 {
			report, err := service.Get(t.Context(), owner.User.ID, statistics.Days, time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			var sum int64
			for _, bucket := range report.Buckets {
				sum += bucket.Count
			}
			if sum != report.ThisWeek || sum != report.ThisMonth {
				t.Fatalf("inconsistent snapshot: %+v", report)
			}
		}
		group.Wait()
		close(results)
		for err := range results {
			if err != nil {
				t.Fatal(err)
			}
		}
		report, err := service.Get(t.Context(), owner.User.ID, statistics.Days, time.UTC)
		if err != nil || report.ThisWeek != 20 || report.ThisMonth != 20 {
			t.Fatalf("unexpected final snapshot: %+v, %v", report, err)
		}
	})

	t.Run("HTTP contract, validation and bearer", func(t *testing.T) {
		owner := api.register(t, "stats-http@example.com")
		call := func(query, access string) statisticsJSON {
			response := api.call(t, "GET", "/statistics"+query, "", "", access, nil)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private report must not be cached")
			}
			return decodeTaskResponse[statisticsJSON](t, response, 200)
		}
		result := call("", owner.AccessToken)
		if result.Period != "days" || result.Timezone != "UTC" || len(result.Buckets) != 7 || result.ThisWeek != 0 || result.ThisMonth != 0 {
			t.Fatalf("unexpected default response: %+v", result)
		}
		for _, bucket := range result.Buckets {
			if _, err := time.Parse(time.DateOnly, bucket.StartDate); err != nil || bucket.Count != 0 {
				t.Fatalf("unexpected empty bucket: %+v", bucket)
			}
		}
		result = call("?period=weeks&timezone=Europe%2FMoscow", owner.AccessToken)
		if result.Period != "weeks" || result.Timezone != "Europe/Moscow" || len(result.Buckets) != 8 {
			t.Fatalf("unexpected weekly response: %+v", result)
		}
		for _, query := range []string{"?period=month", "?period=", "?timezone=Local", "?timezone=No/Such_Zone",
			"?timezone=UTC&timezone=UTC", "?period=days&period=weeks", "?userId=" + owner.User.ID, "?timezone=%ZZ"} {
			checkAuthResponse(t, api.call(t, "GET", "/statistics"+query, "", "", owner.AccessToken, nil), 422, "VALIDATION_ERROR")
		}
		cfg := integrationConfig().Auth
		signedByAnotherKey, _, err := token.NewAccess([]byte("invalid-signature-fixture-only"), cfg.Issuer, cfg.Audience, cfg.AccessTTL).Issue(owner.User.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, access := range []string{"", signedByAnotherKey} {
			response := api.call(t, "GET", "/statistics?period=invalid", "", "", access, nil)
			checkAuthResponse(t, response, 401, "UNAUTHORIZED")
			if response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing bearer challenge")
			}
		}
	})

	t.Run("database deadline returns a safe error", func(t *testing.T) {
		owner := api.register(t, "stats-deadline@example.com")
		cfg := integrationConfig()
		cfg.Auth.RequestTimeout = 100 * time.Millisecond
		router, err := app.Router(cfg, pool)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := pool.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(t.Context())
		if _, err := tx.Exec(t.Context(), "LOCK TABLE task_history IN ACCESS EXCLUSIVE MODE"); err != nil {
			t.Fatal(err)
		}

		request := httptest.NewRequestWithContext(t.Context(), "GET", "/api/v1/statistics", nil)
		request.Header.Set("Authorization", "Bearer "+owner.AccessToken)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		checkAuthResponse(t, response, 503, "SERVICE_UNAVAILABLE")
	})
}

func utc(t *testing.T, value string) time.Time {
	t.Helper()
	instant, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		t.Fatal(err)
	}
	return instant
}

func getStatistics(t *testing.T, pool *pgxpool.Pool, owner string, now time.Time, period statistics.Period, name string) statistics.Report {
	t.Helper()
	zone, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	service := statistics.NewService(statistics.NewRepository(pool), func() time.Time { return now })
	report, err := service.Get(t.Context(), owner, period, zone)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func assertStatistics(t *testing.T, report statistics.Report, week, month int64, counts ...int64) {
	t.Helper()
	actual := make([]int64, 0, len(report.Buckets))
	for _, bucket := range report.Buckets {
		actual = append(actual, bucket.Count)
	}
	if week != report.ThisWeek || month != report.ThisMonth || !reflect.DeepEqual(actual, counts) {
		t.Fatalf("got week=%d month=%d buckets=%v, want week=%d month=%d buckets=%v",
			report.ThisWeek, report.ThisMonth, actual, week, month, counts)
	}
}

func dateHistory(t *testing.T, pool *pgxpool.Pool, task string, dates ...string) {
	t.Helper()
	times := make([]time.Time, len(dates))
	for i, date := range dates {
		times[i] = utc(t, date)
	}
	result, err := pool.Exec(t.Context(), `WITH history AS (
		SELECT id, row_number() OVER (ORDER BY changed_at, id) AS n FROM task_history WHERE task_id=$1
	) UPDATE task_history h SET changed_at=($2::timestamptz[])[history.n]
	FROM history WHERE h.id=history.id`, task, times)
	if err != nil || result.RowsAffected() != int64(len(dates)) {
		t.Fatalf("fixture history count mismatch: %v", err)
	}
}

const insertClosureSQL = `WITH closed AS (
	INSERT INTO tasks (user_id, title, status, position, deleted_at)
	VALUES ($1, 'Historical fixture', 'done', NULL, $2) RETURNING id
) INSERT INTO task_history (task_id, new_status, changed_at)
SELECT id, 'done', $2 FROM closed`

func addClosures(t *testing.T, pool *pgxpool.Pool, owner string, events ...string) {
	t.Helper()
	for _, event := range events {
		execSQL(t, pool, insertClosureSQL, owner, utc(t, event))
	}
}
