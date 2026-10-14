package database

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

var (
	// ErrActivationCodeRequired means a selected gated squad has no code.
	ErrActivationCodeRequired = errors.New("squad activation code is required")
	// ErrActivationCodeInvalid means a selected gated squad received a wrong code.
	ErrActivationCodeInvalid = errors.New("squad activation code is invalid")
	// ErrActivationCodeExtra means a code was supplied for an unselected squad.
	ErrActivationCodeExtra = errors.New("squad activation code is extra")
)

func validateSquadActivationCodesTx(ctx context.Context, tx *sql.Tx, combo model.Combo, addons []model.SquadProduct, supplied map[string]string) error {
	required := make(map[string]string)
	for _, squad := range combo.IncludedSquads {
		hash, ok, err := activationRecordTx(ctx, tx, squad.RemnaSquadUUID)
		if err != nil {
			return err
		}
		if ok {
			required[squad.RemnaSquadUUID] = hash
		}
	}
	for _, squad := range addons {
		hash, ok, err := activationRecordTx(ctx, tx, squad.RemnaSquadUUID)
		if err != nil {
			return err
		}
		if ok {
			required[squad.RemnaSquadUUID] = hash
		}
	}
	for rawUUID, rawCode := range supplied {
		uuid := strings.TrimSpace(rawUUID)
		if _, gated := required[uuid]; !gated {
			return ErrActivationCodeExtra
		}
		if strings.TrimSpace(rawCode) == "" {
			return ErrActivationCodeRequired
		}
	}
	for uuid, hash := range required {
		code := strings.TrimSpace(supplied[uuid])
		if code == "" {
			return ErrActivationCodeRequired
		}
		if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)); err != nil {
			return ErrActivationCodeInvalid
		}
	}
	return nil
}

func activationRecordTx(ctx context.Context, tx *sql.Tx, uuid string) (string, bool, error) {
	var required int
	var hash sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT activation_required,activation_code_hash FROM squad_product_overrides WHERE remna_squad_uuid=?`, uuid).Scan(&required, &hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if required != 1 || !hash.Valid || hash.String == "" {
		return "", false, nil
	}
	return hash.String, true, nil
}
