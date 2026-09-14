package dashboard

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/xh-polaris/psych-core-api/biz/infra/mapper/report"
)

func TestBuildRecentWeekTrendPointsEndsWithToday(t *testing.T) {
	// 2026-09-07 is Monday: the recent-seven-day chart must run Tue -> Mon,
	// rather than putting today first and yesterday last.
	today := time.Date(2026, 9, 7, 12, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	counts := map[int32]int32{1: 11, 2: 22, 7: 77}

	points := buildRecentWeekTrendPoints(counts, today)
	wantWeeks := []int32{2, 3, 4, 5, 6, 7, 1}
	if len(points) != len(wantWeeks) {
		t.Fatalf("got %d points, want %d", len(points), len(wantWeeks))
	}
	for i, want := range wantWeeks {
		if points[i].Week != want {
			t.Fatalf("point %d week = %d, want %d", i, points[i].Week, want)
		}
	}
	if points[0].Count != 22 || points[5].Count != 77 || points[6].Count != 11 {
		t.Fatalf("counts were not kept with weekdays: %#v", points)
	}
}

func TestAnalysisToPBEncodesAbsentListsAsEmptyArrays(t *testing.T) {
	analysis := analysisToPB(&report.Analysis{})
	body, err := json.Marshal(analysis)
	if err != nil {
		t.Fatalf("marshal analysis: %v", err)
	}
	if bytes.Contains(body, []byte(":null")) {
		t.Fatalf("analysis contains null list fields: %s", body)
	}
}
