package routingmatch

// StructurallyAcyclic proves that no sequence of managed hops can revisit a
// listener, even if sniffing or DNS changes the matching destination per hop.
// Conditional cycles need the stronger, fixed-metadata proof used by Paths.
func (g *Graph) StructurallyAcyclic() bool {
	state := map[string]uint8{}
	var visit func(string) bool
	visit = func(node string) bool {
		if state[node] == 1 {
			return false
		}
		if state[node] == 2 {
			return true
		}
		state[node] = 1
		for _, index := range g.byNode[node] {
			if !visit(g.rules[index].To) {
				return false
			}
		}
		state[node] = 2
		return true
	}
	for node := range g.byNode {
		if !visit(node) {
			return false
		}
	}
	return true
}
