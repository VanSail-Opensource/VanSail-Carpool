# Managed routing matcher

`model.go` defines the typed domain/IP/port/network/protocol predicate and native
first-match semantics. Categories combine with AND; alternatives within a category
combine with OR. `domains.go` implements full, suffix, keyword and Go regular
expressions, including a caller-supplied, generation-pinned geosite expansion.
`feasibility.go` proves disjoint conditions with compact CIDR and port
intersections and domain containment. Concrete destinations prove overlapping
cycles; bounded or unresolved relationships return uncertainty, never permission.

`graph.go` builds the ordered graph, enumerates separate conditional paths and
traces a destination using the same compiled predicates. Prior matching rules
remain exclusions at later hops. Limits bound rules, paths, selectors and analysis
work. No telemetry or graph state is persisted here. The caller supplies desired
state, entitlement validation and authenticated peer deployment.

`graph_test.go` contains hosted regressions for separate Google/AI paths, ordered
shadowing, disjoint cycles, overlapping cycles, uncertain regexes, CIDRs, ports,
network conjunction and invalid selectors. No external calls occur in this module.

`regex.go` bounds Go syntax-tree repetition expansion before compiling expressions, with a shared per-graph instruction budget. Hosted regressions preserve ordinary Go expression behavior while refusing excessive expansion.

Domain inputs and non-regex patterns use the pinned Xray lowercase rules; regular-expression text retains Go case sensitivity against that lowered input. IPv4-mapped CIDR selectors are refused instead of assuming ambiguous address-family semantics.
