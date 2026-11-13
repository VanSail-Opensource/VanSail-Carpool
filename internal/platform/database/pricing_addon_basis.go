package database

import (
	"context"
	"database/sql"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/pricing/renewal"
)

func pricingAddonTerms(addons []model.SquadProduct) []renewal.AddonTerm {
	out := make([]renewal.AddonTerm, 0, len(addons))
	for _, a := range addons {
		out = append(out, renewal.AddonTerm{ID: a.GroupID, Price: a.PriceTXBMinor})
	}
	return out
}

func paidPricingAddonTermsTx(ctx context.Context, tx *sql.Tx, purchase string) ([]renewal.AddonTerm, error) {
	rows, err := tx.QueryContext(ctx, `SELECT group_id,charged_txb_minor FROM purchase_addons WHERE purchase_id=$1 ORDER BY group_id`, purchase)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []renewal.AddonTerm{}
	for rows.Next() {
		var a renewal.AddonTerm
		if err = rows.Scan(&a.ID, &a.Price); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func currentPricingContractGross(c renewal.Contract, addons []model.SquadProduct) (int64, error) {
	gross := c.Price
	for _, a := range c.Addons {
		if a.Price < 0 || gross < a.Price {
			return 0, ErrConflict
		}
		gross -= a.Price
	}
	for _, a := range addons {
		var err error
		gross, err = checkedPriceSum(gross, a.PriceTXBMinor)
		if err != nil {
			return 0, err
		}
	}
	if gross < 0 {
		return 0, ErrConflict
	}
	return gross, nil
}

func pricingAddonTermsMatch(c renewal.Contract, addons []model.SquadProduct) bool {
	if len(c.Addons) != len(addons) {
		return false
	}
	prices := map[string]int64{}
	for _, a := range c.Addons {
		prices[a.ID] = a.Price
	}
	for _, a := range addons {
		price, ok := prices[a.GroupID]
		if !ok || price != a.PriceTXBMinor {
			return false
		}
	}
	return true
}
