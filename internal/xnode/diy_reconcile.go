package xnode

import (
	"context"
	"encoding/json"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/model"
)

const DIYReconcileJob = "xnode_diy_reconcile"

func (s *Service) ReconcileDIY(ctx context.Context, job model.OutboxJob) error {
	var input struct {
		UserID string `json:"userId"`
	}
	if json.Unmarshal([]byte(job.Payload), &input) != nil || input.UserID == "" {
		return ErrInvalid
	}
	doc, settings, err := s.readDIYSettings(ctx, input.UserID)
	if err != nil || doc.Cipher == "" {
		return err
	}
	quota, err := s.whitelistQuota(ctx, input.UserID)
	if err != nil {
		return err
	}
	previous := settings
	proxies, pending, err := s.memberInventory(ctx, input.UserID)
	if err != nil {
		return err
	}
	if !pending {
		settings.SplitProfiles = validSplitProfiles(settings.SplitProfiles, proxies)
	}
	settings.Whitelist.IPs = settings.Whitelist.IPs[:min(len(settings.Whitelist.IPs), quota)]
	if sameDIYValue(previous, settings) {
		return nil
	}
	doc.Cipher, err = s.seal("xnode.diy."+input.UserID, settings)
	if err != nil {
		return err
	}
	doc.NativeChanged = false // Effective projections already enforce the current quota.
	repo, ok := s.repo.(interface {
		SaveSystemDIY(context.Context, DIYDocument) error
	})
	if !ok {
		return ErrUnavailable
	}
	return repo.SaveSystemDIY(ctx, doc)
}
