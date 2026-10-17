package xnode

import "github.com/VanSail-Opensource/VanSail-Carpool/internal/routingmatch"

// Client labels resolve rule IDs against the returned owner-scoped choices and
// administrator templates. Expanded geosite sets and repeated residual predicates
// stay in memory instead of multiplying the browser response size.
func routingPublicPaths(paths []routingmatch.Path) []routingmatch.Path {
	out := make([]routingmatch.Path, 0, len(paths))
	for _, p := range paths {
		p.Conditions = []routingmatch.Match{}
		p.Exclusions = []routingmatch.Match{}
		out = append(out, p)
	}
	return out
}
