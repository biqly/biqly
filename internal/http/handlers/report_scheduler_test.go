package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/biqly/biqly/internal/metadata"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatalf("parse time %q: %v", value, err)
	}
	return ts
}

func TestReportSlotAt(t *testing.T) {
	tests := []struct {
		name  string
		sched metadata.ReportScheduleRow
		now   string
		want  string
	}{
		{
			name:  "daily before hour uses yesterday",
			sched: metadata.ReportScheduleRow{Cadence: "daily", HourUTC: 7},
			now:   "2026-03-10T06:30:00Z",
			want:  "2026-03-09T07:00:00Z",
		},
		{
			name:  "daily after hour uses today",
			sched: metadata.ReportScheduleRow{Cadence: "daily", HourUTC: 7},
			now:   "2026-03-10T07:00:00Z",
			want:  "2026-03-10T07:00:00Z",
		},
		{
			// 2026-03-10 is a Tuesday (weekday 2); target Monday (1).
			name:  "weekly rolls back to target weekday",
			sched: metadata.ReportScheduleRow{Cadence: "weekly", HourUTC: 8, Weekday: 1},
			now:   "2026-03-10T12:00:00Z",
			want:  "2026-03-09T08:00:00Z",
		},
		{
			name:  "weekly same day before hour uses previous week",
			sched: metadata.ReportScheduleRow{Cadence: "weekly", HourUTC: 8, Weekday: 2},
			now:   "2026-03-10T07:00:00Z",
			want:  "2026-03-03T08:00:00Z",
		},
		{
			name:  "monthly before day uses previous month",
			sched: metadata.ReportScheduleRow{Cadence: "monthly", HourUTC: 6, DayOfMonth: 15},
			now:   "2026-03-10T12:00:00Z",
			want:  "2026-02-15T06:00:00Z",
		},
		{
			name:  "monthly after day uses this month",
			sched: metadata.ReportScheduleRow{Cadence: "monthly", HourUTC: 6, DayOfMonth: 1},
			now:   "2026-03-10T12:00:00Z",
			want:  "2026-03-01T06:00:00Z",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reportSlotAt(tc.sched, mustTime(t, tc.now))
			assert.Equal(t, mustTime(t, tc.want), got)
		})
	}
}

func TestReportScheduleDue(t *testing.T) {
	now := mustTime(t, "2026-03-10T07:05:00Z")
	base := metadata.ReportScheduleRow{
		Cadence:   "daily",
		HourUTC:   7,
		CreatedAt: mustTime(t, "2026-03-01T00:00:00Z"),
	}

	t.Run("never run and slot passed", func(t *testing.T) {
		assert.True(t, reportScheduleDue(base, now))
	})

	t.Run("already ran for current slot", func(t *testing.T) {
		s := base
		s.LastRunAt = new(mustTime(t, "2026-03-10T07:01:00Z"))
		assert.False(t, reportScheduleDue(s, now))
	})

	t.Run("last run before current slot", func(t *testing.T) {
		s := base
		s.LastRunAt = new(mustTime(t, "2026-03-09T07:01:00Z"))
		assert.True(t, reportScheduleDue(s, now))
	})

	t.Run("created after slot waits for next", func(t *testing.T) {
		s := base
		s.CreatedAt = mustTime(t, "2026-03-10T07:02:00Z")
		assert.False(t, reportScheduleDue(s, now))
	})

	t.Run("slot not reached yet", func(t *testing.T) {
		s := base
		s.HourUTC = 9
		s.LastRunAt = new(mustTime(t, "2026-03-09T09:01:00Z"))
		assert.False(t, reportScheduleDue(s, now))
	})
}
