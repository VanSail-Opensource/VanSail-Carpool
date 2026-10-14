package billing

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

type boundedBillingRepository struct {
	*billingRepository
	bounds model.AddTXBBounds
	err    error
}

func (r *boundedBillingRepository) AddTXBBounds(context.Context) (model.AddTXBBounds, error) {
	return r.bounds, r.err
}

func TestValidateAddTXBAmountUsesPersistedBounds(t *testing.T) {
	t.Parallel()
	repository := &boundedBillingRepository{billingRepository: newBillingRepository(),
		bounds: model.AddTXBBounds{MinimumTXBMinor: 250, MaximumTXBMinor: 5_000}}
	service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})
	tests := []struct {
		name    string
		amount  int64
		wantErr error
	}{
		{name: "below minimum", amount: 249, wantErr: ErrInvalidOrder},
		{name: "minimum inclusive", amount: 250},
		{name: "maximum inclusive", amount: 5_000},
		{name: "above maximum", amount: 5_001, wantErr: ErrInvalidOrder},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := service.validateAddTXBAmount(context.Background(), test.amount)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("validateAddTXBAmount(%d) error = %v, want %v", test.amount, err, test.wantErr)
			}
		})
	}
}

func TestValidateAddTXBAmountFailsClosedWhenBoundsCannotLoad(t *testing.T) {
	t.Parallel()
	want := errors.New("bounds unavailable")
	repository := &boundedBillingRepository{billingRepository: newBillingRepository(), err: want}
	service := newBillingServiceForTest(repository, &billingSettings{}, &billingGateway{})
	if err := service.validateAddTXBAmount(context.Background(), 500); !errors.Is(err, want) {
		t.Fatalf("validateAddTXBAmount() error = %v, want %v", err, want)
	}
}
