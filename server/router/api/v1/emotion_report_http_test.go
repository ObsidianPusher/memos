package v1

import (
	"testing"
	"time"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

func TestParseEmotionReportWindow(t *testing.T) {
	start, end, err := parseEmotionReportWindow("2026-07-01T00:00:00+08:00", "2026-07-07T23:59:59+08:00")
	if err != nil {
		t.Fatalf("parseEmotionReportWindow returned error: %v", err)
	}

	expectedStart, err := time.Parse(time.RFC3339, "2026-07-01T00:00:00+08:00")
	if err != nil {
		t.Fatalf("failed to parse expected start: %v", err)
	}
	expectedEnd, err := time.Parse(time.RFC3339, "2026-07-07T23:59:59+08:00")
	if err != nil {
		t.Fatalf("failed to parse expected end: %v", err)
	}

	if !start.Equal(expectedStart) {
		t.Fatalf("unexpected start time: got %s want %s", start, expectedStart)
	}
	if !end.Equal(expectedEnd) {
		t.Fatalf("unexpected end time: got %s want %s", end, expectedEnd)
	}
}

func TestSummarizeEmotionReport(t *testing.T) {
	location := time.FixedZone("UTC+8", 8*60*60)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, location)
	end := time.Date(2026, 7, 14, 23, 59, 59, 0, location)

	buckets := buildEmotionReportBuckets(start, end, "week")
	memos := []*store.Memo{
		{
			CreatedTs: time.Date(2026, 7, 2, 9, 0, 0, 0, location).Unix(),
			Payload:   &storepb.MemoPayload{Emotion: storepb.MemoPayload_AWESOME},
		},
		{
			CreatedTs: time.Date(2026, 7, 3, 9, 0, 0, 0, location).Unix(),
			Payload:   &storepb.MemoPayload{Emotion: storepb.MemoPayload_GOOD},
		},
		{
			CreatedTs: time.Date(2026, 7, 9, 9, 0, 0, 0, location).Unix(),
			Payload:   &storepb.MemoPayload{Emotion: storepb.MemoPayload_BAD},
		},
		{
			CreatedTs: time.Date(2026, 8, 1, 9, 0, 0, 0, location).Unix(),
			Payload:   &storepb.MemoPayload{Emotion: storepb.MemoPayload_TERRIBLE},
		},
	}

	report := summarizeEmotionReport(memos, buckets, false)
	if report.Total != 3 {
		t.Fatalf("unexpected total: got %d want %d", report.Total, 3)
	}
	if report.DominantEmotion != storepb.MemoPayload_BAD.String() {
		t.Fatalf("unexpected dominant emotion: got %s want %s", report.DominantEmotion, storepb.MemoPayload_BAD.String())
	}
	if len(report.Breakdown) != 5 {
		t.Fatalf("unexpected breakdown length: got %d want %d", len(report.Breakdown), 5)
	}
	if len(report.Series) != 2 {
		t.Fatalf("unexpected series length: got %d want %d", len(report.Series), 2)
	}
	if report.Series[0].Count != 2 {
		t.Fatalf("unexpected first bucket count: got %d want %d", report.Series[0].Count, 2)
	}
	if report.Series[1].Count != 1 {
		t.Fatalf("unexpected second bucket count: got %d want %d", report.Series[1].Count, 1)
	}
}
