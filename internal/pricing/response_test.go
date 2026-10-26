package pricing

import (
	"context"
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

func responseFixture() []ResponseObservation {
	random := rand.New(rand.NewPCG(51, 26))
	rows := make([]ResponseObservation, 2048)
	for i := range rows {
		x := float64(i%101)/100 - .5
		p := 1 / (1 + math.Exp(-(0.5 - 5*x)))
		rows[i] = ResponseObservation{LogPrice: x, Converted: random.Float64() < p}
	}
	return rows
}
func TestResponseChronologicalHoldoutAndPriceVariation(t *testing.T) {
	m := FitResponse(responseFixture())
	if !m.Validated || m.Holdout < 400 || m.Brier >= m.BaselineBrier || m.Coefficients[1] >= 0 {
		t.Fatal(m)
	}
	rows := responseFixture()
	for i := range rows {
		rows[i].LogPrice = 0
	}
	if FitResponse(rows).Validated {
		t.Fatal("no price variation was treated as elasticity evidence")
	}
}
func BenchmarkPricingBoundedFit(b *testing.B) {
	rows := responseFixture()
	b.ReportAllocs()
	for b.Loop() {
		FitResponse(rows)
	}
}
func BenchmarkPricingCachedRead(b *testing.B) {
	c := NewAnalysisCache()
	ctx := context.Background()
	load := func() ([]byte, error) { return []byte("snapshot"), nil }
	c.Load(ctx, "stable", time.Hour, load)
	b.ReportAllocs()
	for b.Loop() {
		c.Load(ctx, "stable", time.Hour, load)
	}
}
