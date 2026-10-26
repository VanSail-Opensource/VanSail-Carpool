package app

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type computePressureSampler struct {
	at          time.Time
	usage, idle uint64
	mode        string
	high        int
	lowSince    time.Time
	pressured   bool
}

func (s *computePressureSampler) observe(now time.Time) bool {
	usage, idle, budget, mode, ok := computeCPUUsage()
	previous, previousIdle, at, oldMode := s.usage, s.idle, s.at, s.mode
	s.usage, s.idle, s.at, s.mode = usage, idle, now, mode
	if !ok || at.IsZero() || mode != oldMode || usage < previous || !now.After(at) {
		return s.pressured
	}
	cpu := 0.0
	if mode == "cgroup" {
		cpu = float64(usage-previous) / (now.Sub(at).Seconds() * 1e6 * budget)
	} else {
		if idle < previousIdle || usage == previous {
			return s.pressured
		}
		cpu = 1 - float64(idle-previousIdle)/float64(usage-previous)
	}
	return s.update(cpu, now)
}

func (s *computePressureSampler) update(cpu float64, now time.Time) bool {
	if cpu >= .9 {
		s.high++
	} else {
		s.high = 0
	}
	if s.high >= 2 {
		s.pressured = true
		s.lowSince = time.Time{}
	}
	if s.pressured && cpu < .75 {
		if s.lowSince.IsZero() {
			s.lowSince = now
		}
		if now.Sub(s.lowSince) >= 30*time.Second {
			s.pressured = false
			s.high = 0
		}
	} else {
		s.lowSince = time.Time{}
	}
	return s.pressured
}

func computeCPUUsage() (uint64, uint64, float64, string, bool) {
	stat, e1 := os.ReadFile("/sys/fs/cgroup/cpu.stat")
	quota, e2 := os.ReadFile("/sys/fs/cgroup/cpu.max")
	if e1 == nil && e2 == nil {
		q := strings.Fields(string(quota))
		if len(q) == 2 && q[0] != "max" {
			limit, e1 := strconv.ParseFloat(q[0], 64)
			period, e2 := strconv.ParseFloat(q[1], 64)
			fields := strings.Fields(string(stat))
			for i := 0; i+1 < len(fields); i += 2 {
				if fields[i] == "usage_usec" {
					usage, err := strconv.ParseUint(fields[i+1], 10, 64)
					if err == nil && e1 == nil && e2 == nil && limit > 0 && period > 0 {
						return usage, 0, limit / period, "cgroup", true
					}
				}
			}
		}
	}
	stat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, 0, "", false
	}
	line := strings.Fields(strings.SplitN(string(stat), "\n", 2)[0])
	if len(line) < 5 || line[0] != "cpu" {
		return 0, 0, 0, "", false
	}
	var total, idle uint64
	// guest/guest_nice are already included in user/nice.
	for i, field := range line[1:min(len(line), 9)] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, 0, 0, "", false
		}
		total += v
		if i == 3 || i == 4 {
			idle += v
		}
	}
	return total, idle, 1, "host", true
}
