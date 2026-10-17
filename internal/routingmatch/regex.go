package routingmatch

import (
	"errors"
	"regexp"
	"regexp/syntax"
)

var errAnalysisBudget = errors.New("regular expression exceeds bounded analysis")

// Parse before compiling: nested counted repetitions can expand a short valid
// Go expression into a very large program. Bound the expanded instruction cost
// while retaining Go's own syntax and runtime matching behavior.
func boundedRegex(expression string) (*regexp.Regexp, int, error) {
	tree, err := syntax.Parse(expression, syntax.Perl)
	if err != nil {
		return nil, 0, err
	}
	cost, ok := regexCost(tree, 0)
	if !ok || cost > 8192 {
		return nil, 0, errAnalysisBudget
	}
	compiled, err := regexp.Compile(expression)
	return compiled, cost, err
}
func regexCost(tree *syntax.Regexp, depth int) (int, bool) {
	if depth > 64 {
		return 0, false
	}
	cost := len(tree.Rune) + 4
	for _, child := range tree.Sub {
		n, ok := regexCost(child, depth+1)
		if !ok {
			return 0, false
		}
		cost += n
		if cost > 8192 {
			return 0, false
		}
	}
	if tree.Op == syntax.OpRepeat {
		factor := tree.Max
		if factor < 0 {
			factor = tree.Min + 1
		}
		factor = max(1, factor)
		if cost > 8192/factor {
			return 0, false
		}
		cost *= factor
	}
	return cost, cost <= 8192
}
