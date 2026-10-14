package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	// ErrPurchaseNotActive means an add-on change cannot alter the requested term.
	ErrPurchaseNotActive = errors.New("purchase is not active")
	// ErrQueuedPurchase means a separately scheduled term owns the next boundary.
	ErrQueuedPurchase = errors.New("queued purchase prevents squad addition")
	// ErrSquadAlreadyAdded means the active term already contains a selected squad.
	ErrSquadAlreadyAdded = errors.New("squad is already added to purchase")
)

type pricedPurchaseAddon struct {
	product model.SquadProduct
	price   int64
}

func quotePurchaseAddonsTx(ctx context.Context, tx *sql.Tx, input PurchaseAddonInput, now time.Time) (model.PurchaseAddonQuote, []pricedPurchaseAddon, error) {
	purchase, err := scanPurchase(tx.QueryRowContext(ctx, purchaseSelect+` WHERE purchases.id=? AND purchases.user_id=?`, input.PurchaseID, input.UserID))
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	if (purchase.Status != "active" && purchase.Status != "activating") || !purchase.ValidUntil.After(now) {
		return model.PurchaseAddonQuote{}, nil, ErrPurchaseNotActive
	}
	var queued, rollover int
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchases WHERE user_id=? AND status='queued')`, input.UserID).Scan(&queued); err != nil {
		return model.PurchaseAddonQuote{}, nil, fmt.Errorf("check queued purchase: %w", err)
	}
	if queued == 1 {
		return model.PurchaseAddonQuote{}, nil, ErrQueuedPurchase
	}
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM purchase_rollovers WHERE purchase_id=?)`, purchase.ID).Scan(&rollover); err != nil {
		return model.PurchaseAddonQuote{}, nil, fmt.Errorf("check purchase rollover: %w", err)
	}
	if rollover == 1 {
		return model.PurchaseAddonQuote{}, nil, ErrPurchaseNotActive
	}
	combo, err := comboByIDTx(ctx, tx, purchase.ComboID, false)
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	existingSquads, err := purchaseSquadsFrom(ctx, tx, purchase.ID)
	if err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	existing := make(map[string]struct{}, len(existingSquads))
	for _, squadUUID := range existingSquads {
		existing[squadUUID] = struct{}{}
	}
	selected := make([]string, 0, len(input.AddonSquadIDs))
	products := make([]model.SquadProduct, 0, len(input.AddonSquadIDs))
	for _, squadID := range input.AddonSquadIDs {
		product, loadErr := squadByIDTx(ctx, tx, squadID, true)
		if loadErr != nil {
			return model.PurchaseAddonQuote{}, nil, loadErr
		}
		if _, found := existing[product.RemnaSquadUUID]; found {
			return model.PurchaseAddonQuote{}, nil, ErrSquadAlreadyAdded
		}
		existing[product.RemnaSquadUUID] = struct{}{}
		selected = append(selected, product.RemnaSquadUUID)
		products = append(products, product)
	}
	if err := checkSquadStockTx(ctx, tx, selected, input.UserID); err != nil {
		return model.PurchaseAddonQuote{}, nil, err
	}
	original := time.Duration(combo.ValidityDays) * 24 * time.Hour
	remaining := purchase.ValidUntil.Sub(now)
	priced := make([]pricedPurchaseAddon, 0, len(products))
	total := int64(0)
	for _, product := range products {
		price, priceErr := proratedAddonPrice(product.PriceTXBMinor, remaining, original)
		if priceErr != nil {
			return model.PurchaseAddonQuote{}, nil, priceErr
		}
		if total > int64(^uint64(0)>>1)-price {
			return model.PurchaseAddonQuote{}, nil, errors.New("purchase add-on price exceeds integer range")
		}
		total += price
		priced = append(priced, pricedPurchaseAddon{product: product, price: price})
	}
	uuids := make([]string, 0, len(priced))
	for _, addon := range priced {
		uuids = append(uuids, addon.product.RemnaSquadUUID)
	}
	return model.PurchaseAddonQuote{PurchaseID: purchase.ID, PriceTXBMinor: total, Price: model.TXBMoney(total), ExpiresAt: purchase.ValidUntil, AddonSquadUUIDs: uuids}, priced, nil
}
