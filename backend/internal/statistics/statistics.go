package statistics

import "time"

type Period string

const (
	Days  Period = "days"
	Weeks Period = "weeks"
)

type Bucket struct {
	StartDate time.Time
	Count     int64
}

type Report struct {
	Period    Period
	Timezone  string
	Buckets   []Bucket
	ThisWeek  int64
	ThisMonth int64
}

type window struct {
	id         int
	start, end time.Time
}

const (
	weekTotal  = -1
	monthTotal = -2
)
