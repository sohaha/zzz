package stress

import (
	"fmt"
	"time"
)

// RequestStat is the saved information about an individual completed HTTP request
type RequestStat struct {
	StartTime       time.Time `json:"startTime"`
	EndTime         time.Time `json:"endTime"`
	Error           error     `json:"error"`
	Proto           string
	Method          string
	URL             string
	Duration        time.Duration `json:"duration"`
	StatusCode      int           `json:"statusCode"`
	DataTransferred int
}

// RequestStatSummary is an aggregate statistical summary of a set of RequestStats
type RequestStatSummary struct {
	endTime              time.Time
	startTime            time.Time
	statusCodes          map[int]int
	avgDuration          time.Duration
	maxDuration          time.Duration
	minDuration          time.Duration
	avgRPS               float64
	avgDataTransferred   int
	maxDataTransferred   int
	minDataTransferred   int
	totalDataTransferred int
}

// CreateRequestsStats creates a statistical summary out of the individual RequestStats
func CreateRequestsStats(requestStats []RequestStat) RequestStatSummary {
	if len(requestStats) == 0 {
		return RequestStatSummary{}
	}

	requestCodes := make(map[int]int)
	summary := RequestStatSummary{
		maxDuration:          requestStats[0].Duration,
		minDuration:          requestStats[0].Duration,
		minDataTransferred:   requestStats[0].DataTransferred,
		statusCodes:          requestCodes,
		startTime:            requestStats[0].StartTime,
		endTime:              requestStats[0].EndTime,
		totalDataTransferred: 0,
	}
	var totalDurations time.Duration //total time of all requests (concurrent is counted)
	nonErrCount := 0
	for i := 0; i < len(requestStats); i++ {
		if requestStats[i].Error != nil {
			continue
		}
		nonErrCount++
		if requestStats[i].Duration > summary.maxDuration {
			summary.maxDuration = requestStats[i].Duration
		}
		if requestStats[i].Duration < summary.minDuration || summary.minDuration == 0 { //in case was set to 0 due to an error req
			summary.minDuration = requestStats[i].Duration
		}
		if requestStats[i].StartTime.Before(summary.startTime) {
			summary.startTime = requestStats[i].StartTime
		}
		if requestStats[i].EndTime.After(summary.endTime) {
			summary.endTime = requestStats[i].EndTime
		}
		totalDurations += requestStats[i].Duration

		if requestStats[i].DataTransferred > summary.maxDataTransferred {
			summary.maxDataTransferred = requestStats[i].DataTransferred
		}
		if requestStats[i].DataTransferred < summary.minDataTransferred || summary.minDataTransferred == 0 { //in case was set to 0 due to an error req
			summary.minDataTransferred = requestStats[i].DataTransferred
		}
		summary.totalDataTransferred += requestStats[i].DataTransferred

		summary.statusCodes[requestStats[i].StatusCode]++
	}
	if nonErrCount == 0 {
		summary.avgDuration = 0
		summary.maxDuration = 0
		summary.minDuration = 0
		summary.minDataTransferred = 0
		summary.maxDataTransferred = 0
		summary.totalDataTransferred = 0
		return summary
	}
	//kinda ugly to calculate average, then convert into nanoseconds
	avgNs := totalDurations.Nanoseconds() / int64(nonErrCount)
	newAvg, _ := time.ParseDuration(fmt.Sprintf("%d", avgNs) + "ns")
	summary.avgDuration = newAvg

	summary.avgDataTransferred = summary.totalDataTransferred / nonErrCount

	summary.avgRPS = float64(nonErrCount) / float64(summary.endTime.Sub(summary.startTime))
	return summary
}
