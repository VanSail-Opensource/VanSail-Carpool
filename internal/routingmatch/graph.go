package routingmatch

import (
	"fmt"
	"slices"
)

type Path struct {
	ExcludedRules []string `json:"excludedRules"`
	Nodes         []string `json:"nodes"`
	Rules         []string `json:"rules"`
	Conditions    []Match  `json:"conditions"`
	Exclusions    []Match  `json:"exclusions"`
}
type Diagnostic struct {
	Code  string   `json:"code"`
	Rules []string `json:"rules"`
}

func (d *Diagnostic) Error() string { return fmt.Sprintf("%s: %v", d.Code, d.Rules) }

type Graph struct {
	rules    []Rule
	compiled []compiled
	byNode   map[string][]int
}

func Build(rules []Rule, resolve Resolver) (*Graph, error) {
	if len(rules) > 64 {
		return nil, &Diagnostic{Code: "routing_analysis_limit"}
	}
	g := &Graph{rules: slices.Clone(rules), byNode: map[string][]int{}}
	ids := map[string]bool{}
	for i, r := range rules {
		if r.ID == "" || r.From == "" || r.To == "" || r.From == r.To || ids[r.ID] {
			return nil, &Diagnostic{Code: "routing_invalid_rule", Rules: []string{r.ID}}
		}
		ids[r.ID] = true
		c, err := compile(r.Match, resolve)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", r.ID, err)
		}
		c.id = r.ID
		g.compiled = append(g.compiled, c)
		g.byNode[r.From] = append(g.byNode[r.From], i)
	}
	return g, nil
}

// Paths preserves first-match residuals at each hop. The same predicates are
// used by Trace and compilation; topological cycles alone are not rejected.
func (g *Graph) Paths() ([]Path, error) {
	paths := []Path{}
	steps := 0
	budget := 500000
	var visit func(Path, []compiled, []compiled) error
	visit = func(path Path, positive, negative []compiled) error {
		steps++
		if steps > 2048 || len(path.Nodes) > 32 {
			return &Diagnostic{Code: "routing_analysis_limit", Rules: path.Rules}
		}
		node := path.Nodes[len(path.Nodes)-1]
		prior := slices.Clone(negative)
		for _, index := range g.byNode[node] {
			r, c := g.rules[index], g.compiled[index]
			pos := append(slices.Clone(positive), c)
			state := feasible(pos, prior, &budget)
			if budget < 0 {
				return &Diagnostic{Code: "routing_analysis_limit", Rules: path.Rules}
			}
			if state != impossible {
				next := Path{Nodes: append(slices.Clone(path.Nodes), r.To), Rules: append(slices.Clone(path.Rules), r.ID), Conditions: append(slices.Clone(path.Conditions), r.Match), Exclusions: matches(prior), ExcludedRules: excludedRuleIDs(prior)}
				if slices.Contains(path.Nodes, r.To) {
					code := "routing_loop"
					if state == unknown {
						code = "routing_overlap_unproven"
					}
					return &Diagnostic{Code: code, Rules: next.Rules}
				}
				if err := visit(next, pos, slices.Clone(prior)); err != nil {
					return err
				}
			}
			prior = append(prior, c)
		}
		if len(path.Rules) > 0 && feasible(positive, prior, &budget) != impossible {
			path.Exclusions = matches(prior)
			path.ExcludedRules = excludedRuleIDs(prior)
			paths = append(paths, path)
			if len(paths) > 256 {
				return &Diagnostic{Code: "routing_analysis_limit", Rules: path.Rules}
			}
		}
		if budget < 0 {
			return &Diagnostic{Code: "routing_analysis_limit", Rules: path.Rules}
		}
		return nil
	}
	starts := []string{}
	for _, r := range g.rules {
		if !slices.Contains(starts, r.From) {
			starts = append(starts, r.From)
		}
	}
	for _, start := range starts {
		if err := visit(Path{Nodes: []string{start}, Rules: []string{}, Conditions: []Match{}, Exclusions: []Match{}}, nil, nil); err != nil {
			return nil, err
		}
	}
	return paths, nil
}
func matches(values []compiled) []Match {
	out := []Match{}
	for _, c := range values {
		out = append(out, c.match)
	}
	return out
}
func excludedRuleIDs(values []compiled) []string {
	out := []string{}
	for _, c := range values {
		out = append(out, c.id)
	}
	return out
}
func (g *Graph) Trace(start string, d Destination) (Path, error) {
	p := Path{Nodes: []string{start}, Rules: []string{}, Conditions: []Match{}, Exclusions: []Match{}, ExcludedRules: []string{}}
	if d.Port == 0 {
		d.Port = 443
	}
	if d.Network == "" {
		d.Network = "tcp"
	}
	for hops := 0; hops < 32; hops++ {
		node := p.Nodes[len(p.Nodes)-1]
		found := false
		for _, i := range g.byNode[node] {
			if !g.compiled[i].matches(d) {
				continue
			}
			r := g.rules[i]
			if slices.Contains(p.Nodes, r.To) {
				return p, &Diagnostic{Code: "routing_loop", Rules: append(p.Rules, r.ID)}
			}
			p.Nodes = append(p.Nodes, r.To)
			p.Rules = append(p.Rules, r.ID)
			p.Conditions = append(p.Conditions, r.Match)
			found = true
			break
		}
		if !found {
			return p, nil
		}
	}
	return p, &Diagnostic{Code: "routing_analysis_limit", Rules: p.Rules}
}
