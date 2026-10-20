package routingmatch

import "testing"

func TestStructuralProofDoesNotDependOnDestinationPredicates(t *testing.T) {
	for _, test := range []struct {
		name    string
		rules   []Rule
		acyclic bool
	}{
		{"empty", nil, true},
		{"chain", []Rule{rule("ab", "a", "b"), rule("bc", "b", "c")}, true},
		{"diamond", []Rule{rule("ab", "a", "b"), rule("ac", "a", "c"), rule("bd", "b", "d"), rule("cd", "c", "d")}, true},
		{"disjoint conditional cycle", []Rule{rule("ab", "a", "b", "domain:google.com"), rule("ba", "b", "a", "domain:openai.com")}, false},
		{"disconnected cycle", []Rule{rule("ab", "a", "b"), rule("cd", "c", "d"), rule("dc", "d", "c")}, false},
		{"separate listeners", []Rule{rule("ab", "a/one", "b/one"), rule("ba", "b/two", "a/one")}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			graph, err := Build(test.rules, nil)
			if err != nil {
				t.Fatal(err)
			}
			if graph.StructurallyAcyclic() != test.acyclic {
				t.Fatal("structural proof depended on match conditions or merged listeners")
			}
		})
	}
}
