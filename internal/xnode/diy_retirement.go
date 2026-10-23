package xnode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/subscription"
)

func decodeDIYSettings(raw []byte) (DIYSettings, bool, error) {
	settings := DIYSettings{Version: 2, Whitelist: WhitelistPolicy{IPs: []string{}}, SplitProfiles: []SplitProfile{}}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return settings, false, err
	}
	if header.Version == 2 {
		err := json.Unmarshal(raw, &settings)
		return settings, false, err
	}
	if header.Version != 0 && header.Version != 1 {
		return settings, false, ErrInvalid
	}
	var err error
	if header.Version == 0 {
		raw, err = subscription.RetireTuning(raw)
		if err == nil {
			err = json.Unmarshal(raw, &settings.Overrides)
		}
	} else {
		var document map[string]json.RawMessage
		if err = json.Unmarshal(raw, &document); err != nil {
			return settings, false, err
		}
		document["overrides"], err = subscription.RetireTuning(document["overrides"])
		if err != nil {
			return settings, false, err
		}
		raw, err = json.Marshal(document)
		if err == nil {
			err = json.Unmarshal(raw, &settings)
		}
		settings.Version = 2
	}
	return settings, true, err
}

type diyRetirementRepository interface {
	diyReader
	XNodeDIYUsers(context.Context, string) ([]string, error)
	ReplaceXNodeDIY(context.Context, DIYDocument, string) error
}

// RetireDIYOptions reseals old documents before HTTP admission. Revision fences
// invalidate stale quotes without touching billing, receipts or native state.
func (s *Service) RetireDIYOptions(ctx context.Context) error {
	repo, ok := s.repo.(diyRetirementRepository)
	if !ok {
		return nil
	}
	after := ""
	for {
		users, err := repo.XNodeDIYUsers(ctx, after)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			return nil
		}
		for _, user := range users {
			doc, err := repo.XNodeDIY(ctx, user)
			if err != nil {
				return err
			}
			var raw json.RawMessage
			if err = s.open("xnode.diy."+user, doc.Cipher, &raw); err != nil {
				return err
			}
			settings, changed, err := decodeDIYSettings(raw)
			if err != nil {
				return fmt.Errorf("retire client options: %w", err)
			}
			if changed {
				cipher, err := s.seal("xnode.diy."+user, settings)
				if err != nil {
					return err
				}
				if err = repo.ReplaceXNodeDIY(ctx, doc, cipher); err != nil {
					return err
				}
			}
			after = user
		}
	}
}
