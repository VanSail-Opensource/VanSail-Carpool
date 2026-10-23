package catalog

import (
	"context"
	"errors"
	"strconv"
	"testing"
)

type automaticRenewalSettingsStub struct {
	value string
	err   error
}

func (s automaticRenewalSettingsStub) Optional(context.Context, string) (string, error) {
	return s.value, s.err
}

func TestRolloverCountsTowardAutoRenewalBalance(t *testing.T) {
	t.Parallel()
	testError := errors.New("settings unavailable")
	for _, test := range []struct {
		name     string
		settings AutomaticRenewalSettings
		want     bool
		wantErr  error
	}{
		{name: "default", want: true},
		{name: "blank", settings: automaticRenewalSettingsStub{}, want: true},
		{name: "enabled", settings: automaticRenewalSettingsStub{value: "true"}, want: true},
		{name: "disabled", settings: automaticRenewalSettingsStub{value: "false"}, want: false},
		{name: "invalid", settings: automaticRenewalSettingsStub{value: "invalid"}, wantErr: strconv.ErrSyntax},
		{name: "unavailable", settings: automaticRenewalSettingsStub{err: testError}, wantErr: testError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(&catalogRepository{}, &catalogProxy{}, 0)
			service.SetAutomaticRenewalSettings(test.settings)
			got, err := service.rolloverCountsTowardAutoRenewalBalance(context.Background())
			if !errors.Is(err, test.wantErr) || got != test.want {
				t.Fatalf("rolloverCountsTowardAutoRenewalBalance() = (%t, %v), want (%t, %v)", got, err, test.want, test.wantErr)
			}
		})
	}
}
