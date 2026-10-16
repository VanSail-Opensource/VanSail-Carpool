package app

import (
	"context"
	"errors"
	"testing"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/integrations/remnawave"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

type missingAccountGuard struct {
	required bool
	err      error
	calls    int
}

func (g *missingAccountGuard) XNodeMigrationAccountRequired(context.Context, string) (bool, error) {
	g.calls++
	return g.required, g.err
}

func TestMigrationOmitsOnlyConfirmedUnpurchasedMissingAccounts(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	unavailable := &remnawave.APIError{HTTPStatus: 503}
	for _, test := range []struct {
		name     string
		source   error
		required bool
		lookup   error
		want     error
		calls    int
	}{
		{"absent registration", &remnawave.APIError{HTTPStatus: 404}, false, nil, nil, 1},
		{"purchase or imported identity", &remnawave.APIError{HTTPStatus: 404}, true, nil, xnode.ErrConflict, 1},
		{"accounting lookup failure", &remnawave.APIError{HTTPStatus: 404}, false, databaseErr, databaseErr, 1},
		{"source outage", unavailable, false, nil, unavailable, 0},
		{"cancellation", context.Canceled, false, nil, context.Canceled, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			guard := &missingAccountGuard{required: test.required, err: test.lookup}
			err := missingMigrationAccountError(context.Background(), guard, "panel-user", test.source)
			if !errors.Is(err, test.want) || guard.calls != test.calls {
				t.Fatalf("migration result=%v calls=%d", err, guard.calls)
			}
		})
	}
}
