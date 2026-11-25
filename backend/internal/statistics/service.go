package statistics

import (
	"context"
	"time"
)

type counter interface {
	count(context.Context, string, time.Time, []window) (map[int]int64, error)
}

type Service struct {
	repo counter
	now  func() time.Time
}

func NewService(repo *Repository, now func() time.Time) *Service {
	return &Service{repo: repo, now: now}
}

func (s *Service) Get(ctx context.Context, owner string, period Period, timezone *time.Location) (Report, error) {
	now := s.now()
	report, windows := calendar(now, period, timezone)
	counts, err := s.repo.count(ctx, owner, now, windows)
	if err != nil {
		return Report{}, err
	}
	for i := range report.Buckets {
		report.Buckets[i].Count = counts[i]
	}
	report.ThisWeek, report.ThisMonth = counts[weekTotal], counts[monthTotal]
	return report, nil
}

func calendar(now time.Time, period Period, timezone *time.Location) (Report, []window) {
	year, month, day := now.In(timezone).Date()
	// Represent civil dates in UTC for calendar arithmetic. They are converted
	// to instants only when we intersect them with the timezone's offset periods.
	today := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	monday := today.AddDate(0, 0, -(int(today.Weekday())+6)%7)
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	first, count, stride := monday, 7, 1
	if period == Weeks {
		first, count, stride = monday.AddDate(0, 0, -49), 8, 7
	}

	report := Report{Period: period, Timezone: timezone.String(), Buckets: make([]Bucket, count)}
	windows := make([]window, 0, count+2)
	for i := range report.Buckets {
		start := first.AddDate(0, 0, i*stride)
		report.Buckets[i].StartDate = start
		windows = appendWindows(windows, i, start, start.AddDate(0, 0, stride), timezone)
	}
	windows = appendWindows(windows, weekTotal, monday, monday.AddDate(0, 0, 7), timezone)
	windows = appendWindows(windows, monthTotal, monthStart, monthStart.AddDate(0, 1, 0), timezone)
	return report, windows
}

// A civil date boundary can be skipped or repeated at an offset transition.
// Intersect the civil interval with each fixed-offset segment and pass exact
// instants to PostgreSQL so there is only one source of timezone rules.
func appendWindows(windows []window, id int, start, end time.Time, timezone *time.Location) []window {
	const offsetBound = 24 * time.Hour // Every IANA UTC offset is within this bound.
	limit := end.Add(offsetBound)
	for cursor := start.Add(-offsetBound); cursor.Before(limit); {
		local := cursor.In(timezone)
		_, seconds := local.Zone()
		_, zoneEnd := local.ZoneBounds()
		// Go's extrapolated TZ rules can report the end of a 365-day
		// year on December 31 of a leap year after the final transition.
		if !zoneEnd.IsZero() && !zoneEnd.After(cursor) {
			zoneEnd = time.Date(cursor.Year()+1, 1, 1, 0, 0, 0, 0, time.UTC)
		}
		if zoneEnd.IsZero() || zoneEnd.After(limit) {
			zoneEnd = limit
		}
		offset := time.Duration(seconds) * time.Second
		lower, upper := start.Add(-offset), end.Add(-offset)
		if lower.Before(cursor) {
			lower = cursor
		}
		if upper.After(zoneEnd) {
			upper = zoneEnd
		}
		if lower.Before(upper) {
			windows = append(windows, window{id: id, start: lower.UTC(), end: upper.UTC()})
		}
		cursor = zoneEnd
	}
	return windows
}
