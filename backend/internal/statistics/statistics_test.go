package statistics

import (
	"context"
	"reflect"
	"slices"
	"testing"
	"time"
)

func fixed(t *testing.T, value string) time.Time {
	t.Helper()
	result, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func zone(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestQueryParameters(t *testing.T) {
	for _, test := range []struct {
		query  string
		period Period
		name   string
		field  string
	}{
		{"", Days, "UTC", ""},
		{"period=weeks", Weeks, "UTC", ""},
		{"timezone=Europe%2FMoscow", Days, "Europe/Moscow", ""},
		{"timezone=Europe/Berlin", Days, "Europe/Berlin", ""},
		{"timezone=America/New_York", Days, "America/New_York", ""},
		{"timezone=Asia/Kolkata", Days, "Asia/Kolkata", ""},
		{"timezone=Etc%2FGMT%2B1", Days, "Etc/GMT+1", ""},
		{"period=", "", "", "period"},
		{"period=month", "", "", "period"},
		{"period=days&period=days", "", "", "period"},
		{"timezone=UTC&timezone=Europe/Berlin", "", "", "timezone"},
		{"timezone=", "", "", "timezone"},
		{"timezone=Invalid/City", "", "", "timezone"},
		{"timezone=Local", "", "", "timezone"},
		{"timezone=localtime", "", "", "timezone"},
		{"timezone=posixrules", "", "", "timezone"},
		{"timezone=right/UTC", "", "", "timezone"},
		{"timezone=posix/UTC", "", "", "timezone"},
		{"timezone=/usr/share/zoneinfo/UTC", "", "", "timezone"},
		{"timezone=Europe/../UTC", "", "", "timezone"},
		{"timezone=%GG", "", "", "query"},
		{"userId=example", "", "", "query"},
	} {
		t.Run(test.query, func(t *testing.T) {
			period, loc, fields := queryParameters(test.query)
			if test.field != "" {
				if fields[test.field] == "" {
					t.Fatalf("expected error for %s, got %v", test.field, fields)
				}
				return
			}
			if len(fields) != 0 || period != test.period || loc.String() != test.name {
				t.Fatalf("period=%q timezone=%v errors=%v", period, loc, fields)
			}
		})
	}
}

func TestCivilCalendar(t *testing.T) {
	for _, test := range []struct {
		name, instant, timezone      string
		monday, monthStart, monthEnd string
	}{
		{"sunday", "2026-11-29T23:59:00Z", "UTC", "2026-11-23", "2026-11-01", "2026-12-01"},
		{"monday", "2026-11-30T00:00:00Z", "UTC", "2026-11-30", "2026-11-01", "2026-12-01"},
		{"december to january", "2026-12-31T22:00:00Z", "Europe/Moscow", "2026-12-28", "2027-01-01", "2027-02-01"},
		{"leap day", "2024-02-29T13:00:00Z", "UTC", "2024-02-26", "2024-02-01", "2024-03-01"},
		{"leap year in extrapolated timezone", "2040-12-31T17:00:00Z", "America/New_York", "2040-12-31", "2040-12-01", "2041-01-01"},
		{"non-hour offset", "2026-11-29T19:00:00Z", "Asia/Kolkata", "2026-11-30", "2026-11-01", "2026-12-01"},
	} {
		t.Run(test.name, func(t *testing.T) {
			now, loc := fixed(t, test.instant), zone(t, test.timezone)
			days, dayWindows := calendar(now, Days, loc)
			weeks, weekWindows := calendar(now, Weeks, loc)
			if len(days.Buckets) != 7 || len(weeks.Buckets) != 8 {
				t.Fatal("unexpected series lengths")
			}
			monday := fixed(t, test.monday+"T00:00:00Z")
			for i, bucket := range days.Buckets {
				if !bucket.StartDate.Equal(monday.AddDate(0, 0, i)) || bucket.Count != 0 {
					t.Fatalf("unexpected day: %+v", bucket)
				}
			}
			for i, bucket := range weeks.Buckets {
				if !bucket.StartDate.Equal(monday.AddDate(0, 0, (i-7)*7)) || bucket.Count != 0 {
					t.Fatalf("unexpected week: %+v", bucket)
				}
			}
			var month []window
			for _, w := range dayWindows {
				if w.id == monthTotal {
					month = append(month, w)
				}
			}
			if month[0].start.In(loc).Format(time.DateOnly) != test.monthStart ||
				month[len(month)-1].end.In(loc).Format(time.DateOnly) != test.monthEnd {
				t.Fatalf("incorrect month ranges: %+v", month)
			}
			totals := func(windows []window) []window {
				return slices.DeleteFunc(slices.Clone(windows), func(w window) bool { return w.id >= 0 })
			}
			if !reflect.DeepEqual(totals(dayWindows), totals(weekWindows)) {
				t.Fatal("summary windows depend on period")
			}
		})
	}
}

func TestTimezoneWindows(t *testing.T) {
	for _, test := range []struct {
		zone, date, first string
		hours             time.Duration
	}{
		{"Europe/Berlin", "2026-03-29", "2026-03-28T23:00:00Z", 23},
		{"Europe/Berlin", "2026-10-25", "2026-10-24T22:00:00Z", 25},
		{"America/New_York", "2026-03-08", "2026-03-08T05:00:00Z", 23},
		{"America/New_York", "2026-11-01", "2026-11-01T04:00:00Z", 25},
		{"Asia/Kolkata", "2026-11-30", "2026-11-29T18:30:00Z", 24},
		{"America/Santiago", "2026-09-06", "2026-09-06T04:00:00Z", 23},
		{"Pacific/Apia", "2011-12-30", "", 0},
	} {
		t.Run(test.zone+" "+test.date, func(t *testing.T) {
			start, loc := fixed(t, test.date+"T00:00:00Z"), zone(t, test.zone)
			windows := appendWindows(nil, 0, start, start.AddDate(0, 0, 1), loc)
			var length time.Duration
			for i, w := range windows {
				length += w.end.Sub(w.start)
				if i > 0 && w.start.Before(windows[i-1].end) {
					t.Fatal("overlapping windows")
				}
			}
			if length != test.hours*time.Hour || (test.first != "" && !windows[0].start.Equal(fixed(t, test.first))) {
				t.Fatalf("unexpected ranges: %+v", windows)
			}
		})
	}
}

type counterFunc func(context.Context, string, time.Time, []window) (map[int]int64, error)

func (f counterFunc) count(ctx context.Context, owner string, now time.Time, windows []window) (map[int]int64, error) {
	return f(ctx, owner, now, windows)
}

func TestZeroFillingAndSingleClockRead(t *testing.T) {
	now := fixed(t, "2026-11-25T12:00:00Z")
	clockReads := 0
	service := &Service{
		now: func() time.Time { clockReads++; return now },
		repo: counterFunc(func(_ context.Context, owner string, instant time.Time, windows []window) (map[int]int64, error) {
			if owner != "test-owner" || !instant.Equal(now) || len(windows) != 9 {
				t.Fatal("repository got different request inputs")
			}
			return map[int]int64{0: 2, weekTotal: 2, monthTotal: 4}, nil
		}),
	}
	report, err := service.Get(t.Context(), "test-owner", Days, time.UTC)
	if err != nil || clockReads != 1 || report.ThisWeek != 2 || report.ThisMonth != 4 || report.Buckets[0].Count != 2 {
		t.Fatalf("unexpected report: %+v, %v", report, err)
	}
	for _, bucket := range report.Buckets[1:] {
		if bucket.Count != 0 {
			t.Fatal("missing buckets must have zero counts")
		}
	}
}
