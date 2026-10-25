package xnode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"

	wire "github.com/VanSail-Opensource/X-Node/pkg/protocol"
)

func ddnsJobsForNode(node string, settings NodeSettings, providers map[string]wire.Provider) []wire.DDNSJob {
	jobs := slices.Clone(settings.DDNS)
	for index := range jobs {
		job := &jobs[index]
		job.Generation = ""
		binding := settings.DDNSBindings[job.ID]
		if binding.ConfiguredBy == "" {
			continue
		}
		raw, _ := json.Marshal([]any{node, *job, binding.IPv4HostIDs, binding.IPv6HostIDs, providers[job.Provider]})
		digest := sha256.Sum256(raw)
		job.Generation = hex.EncodeToString(digest[:])
	}
	return jobs
}

func (s *Service) prepareDDNSSettings(ctx context.Context, node string, settings *NodeSettings) error {
	existing, err := s.loadDDNSBindings(ctx, node)
	if err != nil {
		return err
	}
	provided := settings.DDNSBindings != nil
	if !provided {
		settings.DDNSBindings = existing
	}
	jobs := map[string]bool{}
	bindings := map[string]DDNSHostBinding{}
	hosts := map[string]bool{}
	for index := range settings.DDNS {
		job := &settings.DDNS[index]
		job.Generation = "" // never trust or persist a caller-supplied generation
		jobs[job.ID] = true
		binding := settings.DDNSBindings[job.ID]
		var valid bool
		if binding.IPv4HostIDs, valid = canonicalDDNSHostIDs(binding.IPv4HostIDs); !valid {
			return ErrInvalid
		}
		if binding.IPv6HostIDs, valid = canonicalDDNSHostIDs(binding.IPv6HostIDs); !valid {
			return ErrInvalid
		}
		binding.ConfiguredBy = existing[job.ID].ConfiguredBy
		if settings.ddnsActor != "" {
			binding.ConfiguredBy = settings.ddnsActor
		}
		builtin := job.Provider == wire.BuiltinDDNSProvider
		if builtin && strings.TrimSpace(job.Interface) == "" && (job.IPv4 != "" && job.IPv4URL == "" || job.IPv6 != "" && job.IPv6URL == "") {
			return ErrInvalid
		}
		if builtin && (binding.ConfiguredBy == "" || job.IPv4 != "" && len(binding.IPv4HostIDs) == 0 || job.IPv6 != "" && len(binding.IPv6HostIDs) == 0) {
			return ErrInvalid
		}
		if !builtin && (len(binding.IPv4HostIDs) > 1 || len(binding.IPv6HostIDs) > 1) {
			return ErrInvalid
		}
		if len(binding.IPv4HostIDs) > 0 && job.IPv4 == "" || len(binding.IPv6HostIDs) > 0 && job.IPv6 == "" {
			return ErrInvalid
		}
		hostIDs := append(slices.Clone(binding.IPv4HostIDs), binding.IPv6HostIDs...)
		for _, id := range hostIDs {
			if binding.ConfiguredBy == "" || hosts[id] {
				return ErrInvalid
			}
			hosts[id] = true
		}
		if binding.ConfiguredBy != "" {
			bindings[job.ID] = binding
		}
	}
	if provided {
		for id := range settings.DDNSBindings {
			if !jobs[id] {
				return ErrInvalid
			}
		}
	}
	if len(hosts) > 0 {
		endpoints, err := s.repo.XNodeEndpoints(ctx)
		if err != nil {
			return err
		}
		for _, endpoint := range endpoints {
			if endpoint.NodeID == node {
				delete(hosts, endpoint.ID)
			}
		}
		if len(hosts) != 0 {
			return ErrInvalid
		}
	}
	settings.DDNSBindings = bindings
	return nil
}

func canonicalDDNSHostIDs(ids []string) ([]string, bool) {
	result := slices.Clone(ids)
	for _, id := range result {
		if strings.TrimSpace(id) == "" {
			return nil, false
		}
	}
	slices.Sort(result)
	compact := slices.Compact(result)
	return compact, len(compact) == len(result)
}

// Keep ordinary authorization updates compatible with older agents. Unsupported
// built-in jobs wait for upgrade; external jobs keep legacy record behavior.
func ddnsForCapabilities(config wire.Config, capabilities []string, negotiated bool) wire.Config {
	changes := negotiated && containsCapability(capabilities, wire.DDNSChangesCapability)
	builtin := changes && containsCapability(capabilities, wire.BuiltinDDNSCapability)
	jobs := make([]wire.DDNSJob, 0, len(config.DDNS))
	for _, job := range config.DDNS {
		if job.Provider == wire.BuiltinDDNSProvider && !builtin {
			continue
		}
		if !changes {
			job.Generation = ""
		}
		jobs = append(jobs, job)
	}
	config.DDNS = jobs
	return config
}
