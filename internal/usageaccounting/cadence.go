package usageaccounting

import (
	"time"
)

type cadencePeriod struct{ start, end time.Time }

// CadenceAdvance is shared with panel-owned quota period scheduling.
func CadenceAdvance(value time.Time, strategy string) time.Time {
	return cadenceAdvance(value, strategy)
}

func cadencePeriods(anchor, start, end time.Time, strategy string) []cadencePeriod {
	if strategy == "NO_RESET" {
		return []cadencePeriod{{start: start, end: end}}
	}
	periodStart := anchor
	for periodStart.After(start) {
		periodStart = cadencePrevious(periodStart, strategy)
	}
	result := make([]cadencePeriod, 0)
	for periodStart.Before(end) {
		next := cadenceAdvance(periodStart, strategy)
		result = append(result, cadencePeriod{periodStart, next})
		periodStart = next
	}
	return result
}

func cadenceAdvance(value time.Time, strategy string) time.Time {
	switch strategy {
	case "DAY":
		return value.AddDate(0, 0, 1)
	case "WEEK":
		return value.AddDate(0, 0, 7)
	case "MONTH":
		return addCalendarMonth(value, 1)
	case "MONTH_ROLLING":
		return value.Add(30 * 24 * time.Hour)
	default:
		return value.AddDate(0, 0, 1)
	}
}

func cadencePrevious(value time.Time, strategy string) time.Time {
	switch strategy {
	case "DAY":
		return value.AddDate(0, 0, -1)
	case "WEEK":
		return value.AddDate(0, 0, -7)
	case "MONTH":
		return addCalendarMonth(value, -1)
	case "MONTH_ROLLING":
		return value.Add(-30 * 24 * time.Hour)
	default:
		return value.AddDate(0, 0, -1)
	}
}

func addCalendarMonth(value time.Time, months int) time.Time {
	value = value.UTC()
	target := time.Date(value.Year(), time.Month(int(value.Month())+months), 1, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
	targetLast := target.AddDate(0, 1, -1)
	day := value.Day()
	sourceLast := time.Date(value.Year(), value.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day == sourceLast || day > targetLast.Day() {
		day = targetLast.Day()
	}
	return time.Date(target.Year(), target.Month(), day, value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), time.UTC)
}
