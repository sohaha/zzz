package stress

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestCreateRequestsStatsEmpty(t *testing.T) {
	s := CreateRequestsStats(nil)
	if s.avgRPS != 0 {
		t.Errorf("empty stats: avgRPS = %v, want 0", s.avgRPS)
	}
	if s.avgDuration != 0 || s.maxDuration != 0 || s.minDuration != 0 {
		t.Errorf("empty stats: durations should be 0, got %v/%v/%v", s.avgDuration, s.maxDuration, s.minDuration)
	}
	if s.totalDataTransferred != 0 {
		t.Errorf("empty stats: totalDataTransferred = %v, want 0", s.totalDataTransferred)
	}
}

func TestCreateRequestsStatsAllErrors(t *testing.T) {
	base := time.Unix(0, 0)
	stats := []RequestStat{
		{StartTime: base, EndTime: base, Error: errors.New("conn refused"), Duration: 10 * time.Millisecond},
		{StartTime: base, EndTime: base.Add(5 * time.Millisecond), Error: errors.New("timeout"), Duration: 5 * time.Millisecond},
	}
	s := CreateRequestsStats(stats)
	if s.avgRPS != 0 {
		t.Errorf("all-error stats: avgRPS = %v, want 0", s.avgRPS)
	}
	if s.avgDuration != 0 || s.maxDuration != 0 || s.minDuration != 0 {
		t.Errorf("all-error stats: durations should be reset to 0, got %v/%v/%v", s.avgDuration, s.maxDuration, s.minDuration)
	}
}

func TestCreateRequestsStatsMixed(t *testing.T) {
	base := time.Unix(0, 0)
	stats := []RequestStat{
		{StartTime: base, EndTime: base.Add(100 * time.Millisecond), Duration: 100 * time.Millisecond, StatusCode: 200, DataTransferred: 100},
		{StartTime: base, EndTime: base.Add(200 * time.Millisecond), Duration: 200 * time.Millisecond, StatusCode: 404, DataTransferred: 300},
		{StartTime: base, EndTime: base.Add(150 * time.Millisecond), Error: errors.New("boom"), StatusCode: 0, DataTransferred: 0},
	}
	s := CreateRequestsStats(stats)
	if s.avgDuration != 150*time.Millisecond {
		t.Errorf("avgDuration = %v, want 150ms", s.avgDuration)
	}
	if s.maxDuration != 200*time.Millisecond {
		t.Errorf("maxDuration = %v, want 200ms", s.maxDuration)
	}
	if s.minDuration != 100*time.Millisecond {
		t.Errorf("minDuration = %v, want 100ms", s.minDuration)
	}
	if s.avgDataTransferred != 200 {
		t.Errorf("avgDataTransferred = %v, want 200", s.avgDataTransferred)
	}
	if s.totalDataTransferred != 400 {
		t.Errorf("totalDataTransferred = %v, want 400", s.totalDataTransferred)
	}
	if s.statusCodes[200] != 1 || s.statusCodes[404] != 1 {
		t.Errorf("statusCodes = %v, want {200:1, 404:1}", s.statusCodes)
	}
	// 2 个成功请求 / 200ms 总耗时，avgRPS 内部单位是 per-ns
	if math.IsInf(s.avgRPS, 1) || math.IsNaN(s.avgRPS) {
		t.Fatalf("avgRPS = %v, want finite", s.avgRPS)
	}
	want := float64(2) / float64(200*time.Millisecond.Nanoseconds())
	if s.avgRPS != want {
		t.Errorf("avgRPS = %v, want %v", s.avgRPS, want)
	}
}

// TestCreateRequestsStatsZeroElapsed 回归：所有请求瞬间完成（startTime == endTime）
// 时 avgRPS 不应产生 +Inf。
func TestCreateRequestsStatsZeroElapsed(t *testing.T) {
	base := time.Unix(0, 0)
	stats := []RequestStat{
		{StartTime: base, EndTime: base, Duration: 0, StatusCode: 200, DataTransferred: 10},
		{StartTime: base, EndTime: base, Duration: 0, StatusCode: 200, DataTransferred: 20},
	}
	s := CreateRequestsStats(stats)
	if s.avgRPS != 0 {
		t.Errorf("zero-elapsed stats: avgRPS = %v, want 0", s.avgRPS)
	}
	if math.IsInf(s.avgRPS, 1) || math.IsNaN(s.avgRPS) {
		t.Fatalf("zero-elapsed stats: avgRPS = %v, must be finite", s.avgRPS)
	}
}

// TestCreateRequestsStatsSubMillisElapsed 覆盖总耗时极小（不足 1ns 不可能，但
// 保证 elapsed 为极小正值时 avgRPS 仍为有限值）。
func TestCreateRequestsStatsSubMillisElapsed(t *testing.T) {
	base := time.Unix(0, 0)
	stats := []RequestStat{
		{StartTime: base, EndTime: base.Add(time.Nanosecond), Duration: time.Nanosecond, StatusCode: 200},
	}
	s := CreateRequestsStats(stats)
	if math.IsInf(s.avgRPS, 1) || math.IsNaN(s.avgRPS) {
		t.Fatalf("avgRPS = %v, must be finite", s.avgRPS)
	}
	if s.avgRPS != 1 {
		t.Errorf("avgRPS = %v, want 1 (1 req / 1ns)", s.avgRPS)
	}
}

// TestTextSummaryNoInf 确保渲染出的文本不会出现 Inf。
func TestTextSummaryNoInf(t *testing.T) {
	base := time.Unix(0, 0)
	stats := []RequestStat{
		{StartTime: base, EndTime: base, Duration: 0, StatusCode: 200},
	}
	text := CreateTextStressSummary(CreateRequestsStats(stats))
	if containsInf(text) {
		t.Fatalf("summary contains Inf:\n%s", text)
	}
}

func containsInf(s string) bool {
	for _, token := range []string{"Inf", "NaN", "+Inf", "-Inf"} {
		if len(s) >= len(token) {
			for i := 0; i+len(token) <= len(s); i++ {
				if s[i:i+len(token)] == token {
					return true
				}
			}
		}
	}
	return false
}
