package ai

import (
	"strings"
	"testing"
)

func TestLogicalQuerySchemaIncludesGroupByTimeGrain(t *testing.T) {
	if !strings.Contains(LogicalQuerySchema, `"time_grain"`) {
		t.Fatal("LogicalQuerySchema should expose group_by.time_grain")
	}
	if !strings.Contains(LogicalQuerySchema, `"day","week","month","quarter","year"`) {
		t.Fatal("LogicalQuerySchema should list supported time_grain values")
	}
}
