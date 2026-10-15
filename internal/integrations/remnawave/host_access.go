package remnawave

import "slices"

// HostSquads is the newer host ACL from the official HostsSchema. Older panels
// use excludedInternalSquads; both restrictions apply when both are present.
type HostSquads struct {
	Mode   string   `json:"mode"`
	Squads []string `json:"squads"`
}

func (h Host) ValidSquadPolicy() bool {
	return h.InternalSquads == nil || h.InternalSquads.Mode == "EXCLUDE" || h.InternalSquads.Mode == "ALLOW_ONLY" && len(h.InternalSquads.Squads) > 0
}

func (h Host) AllowsSquad(id string) bool {
	if slices.Contains(h.ExcludedInternalSquads, id) {
		return false
	}
	if h.InternalSquads == nil {
		return true
	}
	switch h.InternalSquads.Mode {
	case "ALLOW_ONLY":
		return slices.Contains(h.InternalSquads.Squads, id)
	case "EXCLUDE":
		return !slices.Contains(h.InternalSquads.Squads, id)
	default:
		return false
	}
}

// HostNodeIDs resolves optional host-node metadata through the inbound relation.
func (h Host) HostNodeIDs(nodes []Node) []string {
	if len(h.Nodes) > 0 {
		return slices.Clone(h.Nodes)
	}
	result := []string{}
	if h.Inbound.ConfigProfileInboundUUID == nil {
		return result
	}
	for _, node := range nodes {
		for _, inbound := range node.ConfigProfile.ActiveInbounds {
			if inbound.UUID == *h.Inbound.ConfigProfileInboundUUID {
				result = append(result, node.UUID)
				break
			}
		}
	}
	return result
}
