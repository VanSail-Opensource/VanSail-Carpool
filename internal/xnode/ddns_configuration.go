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
		raw, _ := json.Marshal([]any{node, *job, binding.IPv4HostID, binding.IPv6HostID, providers[job.Provider]})
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
		binding.ConfiguredBy = existing[job.ID].ConfiguredBy
		if settings.ddnsActor != "" {
			binding.ConfiguredBy = settings.ddnsActor
		}
		builtin := job.Provider == wire.BuiltinDDNSProvider
		if builtin && strings.TrimSpace(job.Interface) == "" && (job.IPv4 != "" && job.IPv4URL == "" || job.IPv6 != "" && job.IPv6URL == "") {
			return ErrInvalid
		}
		if builtin && (binding.ConfiguredBy == "" || job.IPv4 != "" && binding.IPv4HostID == "" || job.IPv6 != "" && binding.IPv6HostID == "") {
			return ErrInvalid
		}
		if binding.IPv4HostID != "" && job.IPv4 == "" || binding.IPv6HostID != "" && job.IPv6 == "" {
			return ErrInvalid
		}
		for _, id := range []string{binding.IPv4HostID, binding.IPv6HostID} {
			if id == "" {
				continue
			}
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
