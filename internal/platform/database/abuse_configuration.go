package database

import (
	"context"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/abuse"
	"github.com/VanSail-Opensource/VanSail-Carpool/internal/platform/ids"
)

func (s *Store) Policy(ctx context.Context) (abuse.Policy, error) {
	var item abuse.Policy
	var rawTags string
	var enabled int
	err := s.db.QueryRowContext(ctx, `SELECT outbound_tags,global_enabled,global_limit,streak_seconds,warning_validity_days,warning_cooldown_minutes,revision FROM abuse_policy WHERE id=1`).Scan(&rawTags, &enabled, &item.GlobalLimit, &item.StreakSeconds, &item.WarningValidityDays, &item.WarningCooldownMinutes, &item.Revision)
	if err != nil {
		return item, err
	}
	tags, valid := abuse.NormalizeOutboundTags(strings.Split(rawTags, ","))
	if !valid {
		return item, abuse.ErrInvalid
	}
	item.OutboundTags = tags
	item.GlobalEnabled = enabled == 1
	return item, nil
}
func (s *Store) UpdatePolicy(ctx context.Context, _ string, input abuse.Policy, now time.Time) (abuse.Policy, error) {
	tags, valid := abuse.NormalizeOutboundTags(input.OutboundTags)
	if !valid || input.GlobalLimit < 0 || input.GlobalLimit > abuse.MaxGlobalQPS || input.StreakSeconds < abuse.MinStreakSeconds || input.StreakSeconds > abuse.MaxStreakSeconds || input.WarningValidityDays < 1 || input.WarningValidityDays > abuse.MaxWarningValidityDays || input.WarningCooldownMinutes < 0 || input.WarningCooldownMinutes > abuse.MaxWarningCooldownMinutes || input.Revision < 0 {
		return abuse.Policy{}, abuse.ErrInvalid
	}
	input.OutboundTags = tags
	result, err := s.db.ExecContext(ctx, `UPDATE abuse_policy SET outbound_tags=?,global_enabled=?,global_limit=?,streak_seconds=?,warning_validity_days=?,warning_cooldown_minutes=?,revision=revision+1,updated_at=? WHERE id=1 AND revision=?`, strings.Join(tags, ","), boolInt(input.GlobalEnabled), input.GlobalLimit, input.StreakSeconds, input.WarningValidityDays, input.WarningCooldownMinutes, stamp(now), input.Revision)
	if err != nil {
		return abuse.Policy{}, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return abuse.Policy{}, ErrConflict
	}
	return s.Policy(ctx)
}
func (s *Store) DomainRules(ctx context.Context) ([]abuse.DomainRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,expression,qps_limit,enabled,revision FROM abuse_domain_rules ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []abuse.DomainRule{}
	for rows.Next() {
		var item abuse.DomainRule
		var enabled int
		if err = rows.Scan(&item.ID, &item.Name, &item.Expression, &item.QPSLimit, &enabled, &item.Revision); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) SaveDomainRule(ctx context.Context, _ string, input abuse.DomainRule, now time.Time) (abuse.DomainRule, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Expression = strings.TrimSpace(input.Expression)
	if input.Name == "" || len(input.Name) > abuse.MaxRuleNameLength || input.Expression == "" || len(input.Expression) > abuse.MaxRuleTextLength || input.QPSLimit < 1 || input.QPSLimit > abuse.MaxGlobalQPS || input.Revision < 0 {
		return abuse.DomainRule{}, abuse.ErrInvalid
	}
	if input.ID == "" {
		id, err := ids.New()
		if err != nil {
			return abuse.DomainRule{}, err
		}
		input.ID = id
		_, err = s.db.ExecContext(ctx, `INSERT INTO abuse_domain_rules(id,name,expression,qps_limit,enabled,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, input.ID, input.Name, input.Expression, input.QPSLimit, boolInt(input.Enabled), stamp(now), stamp(now))
		if err != nil {
			return abuse.DomainRule{}, err
		}
		return input, nil
	}
	result, err := s.db.ExecContext(ctx, `UPDATE abuse_domain_rules SET name=?,expression=?,qps_limit=?,enabled=?,revision=revision+1,updated_at=? WHERE id=? AND revision=?`, input.Name, input.Expression, input.QPSLimit, boolInt(input.Enabled), stamp(now), input.ID, input.Revision)
	if err != nil {
		return abuse.DomainRule{}, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return abuse.DomainRule{}, ErrConflict
	}
	for _, item := range mustRules(s, ctx) {
		if item.ID == input.ID {
			return item, nil
		}
	}
	return abuse.DomainRule{}, ErrNotFound
}
func mustRules(s *Store, ctx context.Context) []abuse.DomainRule {
	items, _ := s.DomainRules(ctx)
	return items
}
func (s *Store) DeleteDomainRule(ctx context.Context, _ string, id string, revision int, now time.Time) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM abuse_domain_rules WHERE id=? AND revision=?`, id, revision)
	if err != nil {
		return err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return ErrConflict
	}
	return nil
}
func (s *Store) Whitelist(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT remna_user_id FROM abuse_whitelist ORDER BY remna_user_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var item string
		if err = rows.Scan(&item); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SetWhitelist(ctx context.Context, id string, enabled bool, now time.Time) error {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > abuse.MaxRemoteIDLength || strings.ContainsRune(id, '\x00') {
		return abuse.ErrInvalid
	}
	if enabled {
		_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO abuse_whitelist(remna_user_id,created_at) VALUES(?,?)`, id, stamp(now))
		return err
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM abuse_whitelist WHERE remna_user_id=?`, id)
	return err
}
func (s *Store) PunishmentRules(ctx context.Context) ([]abuse.PunishmentRule, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT action,enabled,incident_threshold,duration_minutes,all_nodes,revision FROM abuse_punishment_rules ORDER BY CASE action WHEN 'warning' THEN 1 WHEN 'ip_ban' THEN 2 WHEN 'subscription_revoke' THEN 3 ELSE 4 END`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []abuse.PunishmentRule{}
	for rows.Next() {
		var item abuse.PunishmentRule
		var enabled, all int
		if err = rows.Scan(&item.Action, &enabled, &item.IncidentThreshold, &item.DurationMinutes, &all, &item.Revision); err != nil {
			return nil, err
		}
		item.Enabled = enabled == 1
		item.AllNodes = all == 1
		out = append(out, item)
	}
	return out, rows.Err()
}
func (s *Store) SavePunishmentRule(ctx context.Context, _ string, input abuse.PunishmentRule, now time.Time) (abuse.PunishmentRule, error) {
	if !input.Action.Valid() || input.IncidentThreshold < 1 || input.IncidentThreshold > abuse.MaxGlobalQPS || input.Revision < 0 {
		return abuse.PunishmentRule{}, abuse.ErrInvalid
	}
	if input.Action.RequiresDuration() && (input.DurationMinutes < 1 || input.DurationMinutes > abuse.MaxWarningCooldownMinutes) {
		return abuse.PunishmentRule{}, abuse.ErrInvalid
	}
	if !input.Action.RequiresDuration() {
		input.DurationMinutes = 0
	}
	result, err := s.db.ExecContext(ctx, `UPDATE abuse_punishment_rules SET enabled=?,incident_threshold=?,duration_minutes=?,all_nodes=?,revision=revision+1,updated_at=? WHERE action=? AND revision=?`, boolInt(input.Enabled), input.IncidentThreshold, input.DurationMinutes, boolInt(input.AllNodes), stamp(now), input.Action, input.Revision)
	if err != nil {
		return abuse.PunishmentRule{}, err
	}
	changed, _ := result.RowsAffected()
	if changed == 0 {
		return abuse.PunishmentRule{}, ErrConflict
	}
	for _, item := range mustPunishments(s, ctx) {
		if item.Action == input.Action {
			return item, nil
		}
	}
	return abuse.PunishmentRule{}, ErrNotFound
}
func mustPunishments(s *Store, ctx context.Context) []abuse.PunishmentRule {
	items, _ := s.PunishmentRules(ctx)
	return items
}
