package statistics

import "time"

type bucketResponse struct {
	StartDate string `json:"startDate"`
	Count     int64  `json:"count"`
}

type statisticsResponse struct {
	Period    Period           `json:"period"`
	Timezone  string           `json:"timezone"`
	Buckets   []bucketResponse `json:"buckets"`
	ThisWeek  int64            `json:"thisWeek"`
	ThisMonth int64            `json:"thisMonth"`
}

func public(report Report) statisticsResponse {
	buckets := make([]bucketResponse, len(report.Buckets))
	for i, bucket := range report.Buckets {
		buckets[i] = bucketResponse{StartDate: bucket.StartDate.Format(time.DateOnly), Count: bucket.Count}
	}
	return statisticsResponse{
		Period: report.Period, Timezone: report.Timezone, Buckets: buckets,
		ThisWeek: report.ThisWeek, ThisMonth: report.ThisMonth,
	}
}
