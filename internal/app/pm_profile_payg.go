package app

import (
	"context"
	"errors"
	"math"
	"math/big"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

// Read the existing minute summaries without provisioning, billing or upstream
// work. Hidden/unavailable squads remain part of the owner's actual totals.
func (r pmProfileReader) paygProfileUsage(ctx context.Context, user string) (string, model.Money, error) {
	asOf := time.Now().UTC().Truncate(time.Minute)
	usage, err := r.store.XNodePAYGUsage(ctx, user, asOf.Add(-xnode.PAYGWindow), asOf)
	if err != nil {
		return "", model.Money{}, err
	}
	traffic := new(big.Int)
	spent := int64(0)
	for _, value := range usage {
		if value.TrafficBytes < 0 || value.SpentMinor < 0 || value.SpentMinor > math.MaxInt64-spent {
			return "", model.Money{}, errors.New("invalid PAYG profile summary")
		}
		traffic.Add(traffic, big.NewInt(value.TrafficBytes))
		spent += value.SpentMinor
	}
	return traffic.String(), model.TXBMoney(spent), nil
}
