package xnode

import (
	"context"
	"database/sql"
	"errors"
)

func frameErrorDetails(err error) []any {
	var fields []any
	var report trafficReportFailure
	if errors.As(err, &report) {
		fields = append(fields, "report_sequence", report.sequence, "sample_count", report.samples)
	}
	var gap trafficSequenceGap
	if errors.As(err, &gap) {
		fields = append(fields, "received_sequence", gap.received, "expected_sequence", gap.expected)
	}
	var period trafficPeriodMissing
	if errors.As(err, &period) {
		fields = append(fields, "missing_period_id", period.period)
	}
	return fields
}

// frameErrorCause is an allowlist: raw SQL, agent payloads and secrets never log.
func frameErrorCause(err error) string {
	for _, item := range []struct {
		err  error
		code string
	}{
		{context.DeadlineExceeded, "deadline"}, {context.Canceled, "cancelled"},
		{sql.ErrNoRows, "missing_record"}, {ErrNotFound, "missing_record"},
		{ErrInvalid, "invalid_input"}, {ErrConflict, "state_conflict"},
		{ErrUnavailable, "unavailable"},
	} {
		if errors.Is(err, item.err) {
			return item.code
		}
	}
	var coded interface{ SQLState() string }
	if errors.As(err, &coded) {
		switch coded.SQLState() {
		case "40001", "40P01", "55P03":
			return "storage_busy"
		case "25006":
			return "storage_readonly"
		case "58030", "53100":
			return "storage_io"
		case "23000", "23502", "23503", "23505", "23514", "23P01":
			return "storage_constraint"
		}
	}
	return "internal_failure"
}
