package pricing

import "testing"

func TestCapacitySafetyWithUnlimitedMachines(t *testing.T) {
	cases := []struct {
		name string
		in   OptimizationInputs
		safe bool
	}{
		{"all unlimited", OptimizationInputs{CapacityUnbounded: true, CostMinor: 2}, true},
		{"finite alongside unlimited", OptimizationInputs{CapacityGB: 10, CommittedGB: 3, DemandGB: 6, CostMinor: 2}, true},
		{"finite overloaded", OptimizationInputs{CapacityGB: 10, CommittedGB: 3, DemandGB: 8, CostMinor: 2}, false},
		{"negative cost", OptimizationInputs{CapacityUnbounded: true, CostMinor: -1}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.CapacitySafe(); got != tc.safe {
				t.Fatalf("safe=%t", got)
			}
		})
	}
}
