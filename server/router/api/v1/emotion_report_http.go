package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	storepb "github.com/usememos/memos/proto/gen/store"
	"github.com/usememos/memos/store"
)

type emotionReportResponse struct {
	Total           int                    `json:"total"`
	AverageScore    float64                `json:"averageScore"`
	DominantEmotion string                 `json:"dominantEmotion"`
	Breakdown       []emotionBreakdownItem `json:"breakdown"`
	Series          []emotionReportPoint   `json:"series"`
}

type emotionBreakdownItem struct {
	Emotion string  `json:"emotion"`
	Count   int     `json:"count"`
	Ratio   float64 `json:"ratio"`
}

type emotionReportPoint struct {
	LabelLines      []string `json:"labelLines"`
	Value           float64  `json:"value"`
	Count           int      `json:"count"`
	DominantEmotion string   `json:"dominantEmotion"`
}

type emotionReportBucket struct {
	start        time.Time
	end          time.Time
	labelLines   []string
	count        int
	scoreSum     float64
	emotionCount map[storepb.MemoPayload_Emotion]int
}

func (s *APIV1Service) getEmotionReport(c echo.Context) error {
	ctx := c.Request().Context()
	currentUser, err := s.getCurrentUserFromRequest(ctx, c.Request())
	if err != nil || currentUser == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})
	}

	startTime, endTime, err := parseEmotionReportWindow(c.QueryParam("start_time"), c.QueryParam("end_time"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	granularity := strings.ToLower(strings.TrimSpace(c.QueryParam("granularity")))
	if granularity == "" {
		granularity = "week"
	}
	if granularity != "week" && granularity != "month" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid granularity",
		})
	}

	rangeFilter := buildEmotionReportTimeFilter(startTime, endTime)
	oldFilter := strings.TrimSpace(c.QueryParam("old_filter"))
	if oldFilter == "" {
		oldFilter = rangeFilter
	} else {
		oldFilter = fmt.Sprintf("(%s) && (%s)", oldFilter, rangeFilter)
	}
	filter := strings.TrimSpace(c.QueryParam("filter"))

	memoFind := &store.FindMemo{
		ExcludeComments: true,
		CreatorID:       &currentUser.ID,
		RowStatus:       ptrRowStatus(store.Normal),
		OrderByPinned:   true,
	}
	if err := s.buildMemoFindWithFilter(ctx, memoFind, oldFilter); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}
	if filter != "" {
		if err := s.validateFilter(ctx, filter); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": err.Error(),
			})
		}
		memoFind.Filter = &filter
	}

	workspaceMemoRelatedSetting, err := s.Store.GetWorkspaceMemoRelatedSetting(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get workspace memo related setting",
		})
	}
	if workspaceMemoRelatedSetting.DisplayWithUpdateTime {
		memoFind.OrderByUpdatedTs = true
	}

	memos, err := s.Store.ListMemos(ctx, memoFind)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list memos",
		})
	}

	buckets := buildEmotionReportBuckets(startTime, endTime, granularity)
	report := summarizeEmotionReport(memos, buckets, workspaceMemoRelatedSetting.DisplayWithUpdateTime)
	return c.JSON(http.StatusOK, report)
}

func parseEmotionReportWindow(startTimeText, endTimeText string) (time.Time, time.Time, error) {
	if startTimeText == "" || endTimeText == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("missing required query parameters: start_time and end_time")
	}
	startTime, err := time.Parse(time.RFC3339, startTimeText)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid start_time")
	}
	endTime, err := time.Parse(time.RFC3339, endTimeText)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid end_time")
	}
	if endTime.Before(startTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("end_time must be after start_time")
	}
	return startTime, endTime, nil
}

func buildEmotionReportTimeFilter(startTime, endTime time.Time) string {
	return fmt.Sprintf(
		"display_time_after == %d && display_time_before == %d",
		startTime.Unix(),
		endTime.Unix(),
	)
}

func buildEmotionReportBuckets(startTime, endTime time.Time, granularity string) []emotionReportBucket {
	buckets := make([]emotionReportBucket, 0)
	cursor := startTime
	if granularity == "month" {
		cursor = time.Date(startTime.Year(), startTime.Month(), 1, 0, 0, 0, 0, startTime.Location())
		for !cursor.After(endTime) {
			bucketStart := cursor
			nextMonthStart := bucketStart.AddDate(0, 1, 0)
			bucketEnd := nextMonthStart.Add(-time.Nanosecond)
			if bucketEnd.After(endTime) {
				bucketEnd = endTime
			}
			buckets = append(buckets, emotionReportBucket{
				start:        bucketStart,
				end:          bucketEnd,
				labelLines:   []string{bucketStart.Format("2006-01")},
				emotionCount: createEmotionCountMap(),
			})
			cursor = nextMonthStart
		}
		return buckets
	}

	cursor = time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	for !cursor.After(endTime) {
		bucketStart := cursor
		bucketEnd := bucketStart.AddDate(0, 0, 7).Add(-time.Nanosecond)
		if bucketEnd.After(endTime) {
			bucketEnd = endTime
		}
		buckets = append(buckets, emotionReportBucket{
			start:        bucketStart,
			end:          bucketEnd,
			labelLines:   []string{bucketStart.Format("01/02"), bucketEnd.Format("01/02")},
			emotionCount: createEmotionCountMap(),
		})
		cursor = cursor.AddDate(0, 0, 7)
	}
	return buckets
}

func summarizeEmotionReport(memos []*store.Memo, buckets []emotionReportBucket, displayWithUpdateTime bool) *emotionReportResponse {
	globalEmotionCount := createEmotionCountMap()
	total := 0
	scoreSum := 0.0

	for _, memo := range memos {
		if memo.Payload == nil {
			continue
		}
		timestamp := memo.CreatedTs
		if displayWithUpdateTime {
			timestamp = memo.UpdatedTs
		}
		currentTime := time.Unix(timestamp, 0)
		bucketIndex := findEmotionReportBucketIndex(currentTime, buckets)
		if bucketIndex < 0 {
			continue
		}

		emotion := memo.Payload.GetEmotion()
		buckets[bucketIndex].count++
		buckets[bucketIndex].scoreSum += emotionScore(emotion)
		buckets[bucketIndex].emotionCount[emotion]++
		globalEmotionCount[emotion]++
		total++
		scoreSum += emotionScore(emotion)
	}

	series := make([]emotionReportPoint, 0, len(buckets))
	for _, bucket := range buckets {
		series = append(series, emotionReportPoint{
			LabelLines:      bucket.labelLines,
			Value:           summarizeEmotionAverage(bucket.count, bucket.scoreSum),
			Count:           bucket.count,
			DominantEmotion: emotionReportDominantEmotion(bucket.emotionCount),
		})
	}

	return &emotionReportResponse{
		Total:           total,
		AverageScore:    summarizeEmotionAverage(total, scoreSum),
		DominantEmotion: emotionReportDominantEmotion(globalEmotionCount),
		Breakdown:       summarizeEmotionBreakdown(globalEmotionCount, total),
		Series:          series,
	}
}

func summarizeEmotionAverage(count int, scoreSum float64) float64 {
	if count == 0 {
		return emotionScore(storepb.MemoPayload_OKAY)
	}
	return scoreSum / float64(count)
}

func summarizeEmotionBreakdown(emotionCount map[storepb.MemoPayload_Emotion]int, total int) []emotionBreakdownItem {
	breakdown := make([]emotionBreakdownItem, 0, len(emotionCount))
	for _, emotion := range []storepb.MemoPayload_Emotion{
		storepb.MemoPayload_TERRIBLE,
		storepb.MemoPayload_BAD,
		storepb.MemoPayload_OKAY,
		storepb.MemoPayload_GOOD,
		storepb.MemoPayload_AWESOME,
	} {
		count := emotionCount[emotion]
		breakdown = append(breakdown, emotionBreakdownItem{
			Emotion: emotion.String(),
			Count:   count,
			Ratio:   summarizeEmotionRatio(count, total),
		})
	}
	return breakdown
}

func summarizeEmotionRatio(count, total int) float64 {
	if total == 0 {
		return 0
	}
	return (float64(count) / float64(total)) * 100
}

func emotionReportDominantEmotion(emotionCount map[storepb.MemoPayload_Emotion]int) string {
	dominant := storepb.MemoPayload_OKAY
	for _, emotion := range []storepb.MemoPayload_Emotion{
		storepb.MemoPayload_TERRIBLE,
		storepb.MemoPayload_BAD,
		storepb.MemoPayload_OKAY,
		storepb.MemoPayload_GOOD,
		storepb.MemoPayload_AWESOME,
	} {
		if emotionCount[emotion] > emotionCount[dominant] {
			dominant = emotion
		}
	}
	return dominant.String()
}

func findEmotionReportBucketIndex(ts time.Time, buckets []emotionReportBucket) int {
	for index, bucket := range buckets {
		if (ts.Equal(bucket.start) || ts.After(bucket.start)) && (ts.Equal(bucket.end) || ts.Before(bucket.end)) {
			return index
		}
	}
	return -1
}

func createEmotionCountMap() map[storepb.MemoPayload_Emotion]int {
	return map[storepb.MemoPayload_Emotion]int{
		storepb.MemoPayload_TERRIBLE: 0,
		storepb.MemoPayload_BAD:      0,
		storepb.MemoPayload_OKAY:     0,
		storepb.MemoPayload_GOOD:     0,
		storepb.MemoPayload_AWESOME:  0,
	}
}

func emotionScore(emotion storepb.MemoPayload_Emotion) float64 {
	switch emotion {
	case storepb.MemoPayload_TERRIBLE:
		return 1
	case storepb.MemoPayload_BAD:
		return 2
	case storepb.MemoPayload_OKAY:
		return 3
	case storepb.MemoPayload_GOOD:
		return 4
	case storepb.MemoPayload_AWESOME:
		return 5
	default:
		return 0
	}
}

func ptrRowStatus(rowStatus store.RowStatus) *store.RowStatus {
	return &rowStatus
}
