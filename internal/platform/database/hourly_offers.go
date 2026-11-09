package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const hourlyOfferColumns = `group_id,enabled,hourly_price_minor,required_hours,revision,created_at,updated_at`

func scanHourlyOffer(row rowScanner) (model.HourlyOffer, error) {
	var offer model.HourlyOffer
	var enabled int
	err := row.Scan(&offer.GroupID, &enabled, &offer.HourlyPriceTXBMinor, &offer.RequiredHours, &offer.Revision, &offer.CreatedAt, &offer.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return offer, ErrNotFound
	}
	if err != nil {
		return offer, err
	}
	offer.Enabled = enabled == 1
	offer.HourlyPrice = model.TXBMoney(offer.HourlyPriceTXBMinor)
	offer.CreatedAt, offer.UpdatedAt = offer.CreatedAt.UTC(), offer.UpdatedAt.UTC()
	return offer, nil
}

// HourlyOffer reads one explicit hourly offer without deriving legacy pricing.
func (s *Store) HourlyOffer(ctx context.Context, groupID string) (model.HourlyOffer, error) {
	return scanHourlyOffer(s.db.QueryRowContext(ctx, `SELECT `+hourlyOfferColumns+` FROM hourly_offers WHERE group_id=$1`, groupID))
}

// ListHourlyOffers returns configured offers only; absent groups remain disabled.
func (s *Store) ListHourlyOffers(ctx context.Context) ([]model.HourlyOffer, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+hourlyOfferColumns+` FROM hourly_offers ORDER BY group_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	offers := []model.HourlyOffer{}
	for rows.Next() {
		offer, err := scanHourlyOffer(rows)
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}
	return offers, rows.Err()
}
