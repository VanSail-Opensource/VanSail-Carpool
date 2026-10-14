package statistics

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/providerops"
)

var standaloneMultiplier = regexp.MustCompile(`(^|[^[:alnum:]_.])([0-9]+\.[0-9]+)x([^[:alnum:]_.]|$)`)

// HostUpdateRepository persists scheduled provider mutations before execution.
type HostUpdateRepository interface {
	CreateProviderOperation(context.Context, providerops.CreateInput, time.Time) (providerops.Operation, bool, error)
}

type hostRemarkTarget struct {
	HostUUID string `json:"hostUuid"`
	Remark   string `json:"remark"`
}

// QueueHostMultiplierUpdates records changed remarks for hosts linked to
// exactly one known node. Individual failures never prevent remaining hosts.
func QueueHostMultiplierUpdates(ctx context.Context, provider Provider, repository HostUpdateRepository, actorID string, now time.Time) error {
	nodes, err := provider.Nodes(ctx)
	if err != nil {
		return fmt.Errorf("list nodes for host rewrite: %w", err)
	}
	hosts, err := provider.Hosts(ctx)
	if err != nil {
		return fmt.Errorf("list hosts for multiplier rewrite: %w", err)
	}
	multipliers := make(map[string]string, len(nodes))
	for _, node := range nodes {
		if multiplier, ok := formatMultiplierToken(node.Multiplier); ok {
			multipliers[node.UUID] = multiplier
		}
	}
	var failures []error
	for _, host := range hosts {
		if len(host.Nodes) != 1 {
			continue
		}
		multiplier, ok := multipliers[host.Nodes[0]]
		if !ok {
			continue
		}
		next, changed := replaceFirstMultiplier(host.Remark, multiplier)
		if !changed || next == host.Remark {
			continue
		}
		target, fingerprint, err := encodeHostRemarkTarget(host.UUID, next)
		if err != nil {
			failures = append(failures, fmt.Errorf("encode host %s update: %w", host.UUID, err))
			continue
		}
		window := now.UTC().Truncate(30 * time.Minute).Unix()
		key := fmt.Sprintf("host-remark:%d:%s:%s", window, host.UUID, fingerprint[:12])
		_, _, err = repository.CreateProviderOperation(ctx, providerops.CreateInput{
			ActorUserID: actorID, Kind: providerops.KindHostRemarkUpdate, IdempotencyKey: key,
			RequestFingerprint: fingerprint, SealedTarget: target,
			Items: []providerops.ItemInput{{Key: "host", TargetType: "remnawave_host", TargetID: host.UUID}},
		}, now.UTC())
		if err != nil {
			failures = append(failures, fmt.Errorf("queue host %s update: %w", host.UUID, err))
		}
	}
	return errors.Join(failures...)
}

func formatMultiplierToken(value float64) (string, bool) {
	if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", false
	}
	token := strconv.FormatFloat(value, 'f', -1, 64)
	if !strings.Contains(token, ".") {
		token += ".0"
	}
	return token, true
}

func encodeHostRemarkTarget(hostUUID, remark string) (string, string, error) {
	payload, err := json.Marshal(hostRemarkTarget{HostUUID: strings.TrimSpace(hostUUID), Remark: remark})
	if err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(append([]byte("host-remark:v1:"), payload...))
	return base64.RawURLEncoding.EncodeToString(payload), hex.EncodeToString(digest[:]), nil
}

func decodeHostRemarkTarget(value string) (hostRemarkTarget, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return hostRemarkTarget{}, err
	}
	var target hostRemarkTarget
	if err := json.Unmarshal(payload, &target); err != nil {
		return hostRemarkTarget{}, err
	}
	target.HostUUID = strings.TrimSpace(target.HostUUID)
	if target.HostUUID == "" || strings.TrimSpace(target.Remark) == "" || len([]rune(target.Remark)) > 100 {
		return hostRemarkTarget{}, errors.New("invalid host remark target")
	}
	return target, nil
}

func replaceFirstMultiplier(value, replacement string) (string, bool) {
	match := standaloneMultiplier.FindStringSubmatchIndex(value)
	if match == nil {
		return value, false
	}
	return value[:match[4]] + replacement + value[match[5]:], true
}
