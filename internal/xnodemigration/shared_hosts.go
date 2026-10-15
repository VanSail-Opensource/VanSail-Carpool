package xnodemigration

import (
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/VanSail-Opensource/VanSail-Carpool/internal/xnode"
)

var ErrSharedEndpoint = errors.New("shared SS-2022 host requires compatible isolated endpoints")

// Unique native SS-2022 user keys differ from legacy keys. A shared address
// cannot serve both credentials while any peer still belongs to the source.
func sharedSS2022Hosts(snapshot Snapshot, target string, native map[string]bool) ([]Issue, error) {
	targetProfile := ""
	for _, node := range snapshot.Nodes {
		if node.ID == target {
			targetProfile = node.ProfileID
		}
	}
	methods := map[string]map[string]string{}
	for _, profile := range snapshot.Profiles {
		var root struct {
			Inbounds []struct {
				Tag      string `json:"tag"`
				Protocol string `json:"protocol"`
				Settings struct {
					Method string `json:"method"`
				} `json:"settings"`
			} `json:"inbounds"`
		}
		if json.Unmarshal(profile.Config, &root) != nil {
			if target != "" && profile.UUID == targetProfile {
				return nil, xnode.ErrInvalid
			}
			continue
		}
		methods[profile.UUID] = map[string]string{}
		for _, inbound := range root.Inbounds {
			if inbound.Protocol == "shadowsocks" {
				methods[profile.UUID][inbound.Tag] = inbound.Settings.Method
			}
		}
	}
	bindings := map[string]map[string]string{}
	ss2022 := map[string]bool{}
	for _, node := range snapshot.Nodes {
		if node.ID == target && len(node.Inbounds) > 0 && methods[node.ProfileID] == nil {
			return nil, xnode.ErrInvalid
		}
		bindings[node.ID] = map[string]string{}
		for _, inbound := range node.Inbounds {
			bindings[node.ID][inbound.UUID] = inbound.Tag
			if strings.HasPrefix(methods[node.ProfileID][inbound.Tag], "2022-") {
				ss2022[inbound.UUID] = true
			}
		}
	}
	type hostEntry struct{ id, address string }
	hosts := []hostEntry{}
	peers := map[string]map[string]bool{}
	for _, raw := range snapshot.Hosts {
		var host struct {
			ID       string   `json:"uuid"`
			Address  string   `json:"address"`
			Port     int      `json:"port"`
			Nodes    []string `json:"nodes"`
			Disabled bool     `json:"isDisabled"`
			Inbound  struct {
				ID *string `json:"configProfileInboundUuid"`
			} `json:"inbound"`
		}
		if json.Unmarshal(raw, &host) != nil || host.Inbound.ID == nil {
			return nil, xnode.ErrInvalid
		}
		if host.Disabled || !ss2022[*host.Inbound.ID] {
			continue
		}
		nodes := hostBindingNodes(host.Nodes, *host.Inbound.ID, bindings)
		// An unknown explicit peer must not be treated as an already migrated node.
		if len(host.Nodes) > 0 {
			nodes = slices.Clone(host.Nodes)
		}
		address := strings.TrimSuffix(strings.ToLower(host.Address), ".")
		key := address + "\x00" + strconv.Itoa(host.Port)
		if peers[key] == nil {
			peers[key] = map[string]bool{}
		}
		for _, node := range nodes {
			peers[key][node] = true
		}
		hosts = append(hosts, hostEntry{id: host.ID, address: key})
	}
	issues := []Issue{}
	for _, host := range hosts {
		nodes := peers[host.address]
		if len(nodes) < 2 || target != "" && !nodes[target] {
			continue
		}
		unsafe := target == ""
		for node := range nodes {
			if node != target && !native[node] {
				unsafe = true
			}
		}
		if unsafe {
			issues = append(issues, Issue{ResourceID: host.id, Code: "SHARED_SS2022_HOST", Blocking: true})
		}
	}
	return issues, nil
}
