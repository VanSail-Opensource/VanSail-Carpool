# Member routing resilience

The member's saved custom routing (managed `RoutingPlan` graphs, and legacy
`Customization` rules for owners without a managed plan) must keep applying
across administrator Xray configuration, snippet, certificate and host edits and
across fixed-term, PAYG traffic-credit and hourly renewal boundaries.

## Contract

1. **Keep the last applied routing until the replacement is applied.** A
   dependency digest is evidence, not an authorization decision. When digest
   drift is the only change, the compiler emits the rules re-derived from
   current inventory and reconciliation re-prepares the saved generation. A
   dependency-only refresh does not cross the revocation barrier: counters are
   sealed, then the replacement is applied, so rules never disappear between
   revoke and apply.
2. **Drift never disables.** Reconcile re-queues a refreshed generation when an
   active plan's dependencies change, and reactivates a
   `routing_dependencies_changed` plan once its choices resolve again,
   including legacy node-only choices, which the refresh pins to the current
   endpoints. A graph that cannot be validated at all is retried after the
   transient configuration, asset or inventory gap; the compiler fails closed
   (no rules) meanwhile, but the plan stays addressable.
3. **Only durable access loss prunes or disables.** Access pruning still uses
   authoritative endpoint/entitlement inventory, but a renewal boundary is not
   durable loss. While a paid successor is queued or activating, or an enabled
   automatic renewal may still settle (settlement can legitimately be deferred;
   the failure path clears `auto_renew_enabled`), effective-purchase combo
   choices are retained. PAYG and hourly references stay governed by their own
   accounts and enrollments, whose inventories are authoritative.

## Failure paths repaired

| Path | Before | After |
| --- | --- | --- |
| `routing_compile.go` `managedRouting` | Any dependency digest difference omitted the whole graph from the compiled node configuration. | Rules are re-derived from current inventory and applied; only unresolvable choices omit them. |
| `routing_operations.go` `runRoutingOperation` | Any digest difference between prepare and apply disabled the plan with `XNODE_ROUTING_DEPENDENCIES_CHANGED`. | Only an unprovable graph disables the plan. Dependency-only refreshes seal counters and apply directly instead of revoking into a ruleless window that can stall on an offline node. |
| `routing_reconcile.go` `reconcileRoutingPlans` | Disabled active plans on any validation error and disabled non-stable (legacy/node-only) plans on drift, permanently. | Active drift and `routing_dependencies_changed` plans are refreshed; unprovable dependencies are retried. One member's transient queue/preflight failure no longer blocks other members. |
| `routing_access.go` `pruneRoutingAccess` | Pruned choices whenever the live entitlement inventory was empty or partial, including the renewal window where `EffectivePurchase` returns nil. | Prunes only durable loss; renewal continuity keeps combo/legacy choices until settlement. |
| `routing_config_proof.go` `checkRoutingConfig` | Required the saved digest to equal the freshly resolved digest at every apply barrier. | Proves the resolved rules against the sent native configuration; stale digest is a refresh signal, not an apply failure. |

## Preserved safety

- A fully pruned (`len(choices)==0`) or node-removal (`routing_node_removed`)
  plan stays terminal and is never resurrected; removal/purge evidence and
  immutable accounting records are untouched.
- A partial access prune keeps its surviving choices disabled for review
  (`routing_access_removed`); reconcile does not silently undo that review.
- Retained rows grant nothing: compilation still requires an effective
  entitlement, so a member without durable access has no rules and no
  credentials.
- Legacy raw customizations stay suppressed for owners with a managed plan
  (`managedOwners`), so a refresh cannot double-apply routing.

## Residual fail-closed cases

- While `routingInput` cannot validate a saved graph (for example an
  administrator changes `domainStrategy`/sniffing so a cycle proof becomes
  unprovable, a selected template or GeoSite asset is unavailable, or a renewal
  gap empties the member inventory), rules are absent. The plan is kept and
  refreshed automatically after the gap; it is not deleted.
- Hourly routing choices are bound to an enrollment. A new hourly enrollment
  creates new proxy IDs, so a choice pinned to the previous enrollment is
  durably gone and is pruned; the member re-saves routing for the new
  enrollment.
- A dependency refresh rotates the transit credential generation. Counters are
  sealed before the replacement applies; traffic reported after the seal is
  retained but cannot be attributed to the retired generation.

## Tests

- `routing_resilience_test.go` — renewal continuity evidence (including
  deferred auto-renewal settlement), dependency-refresh detection, reconcile
  refresh-vs-terminal decisions, and one failing member not blocking a later
  one.
- `routing_access_test.go` — prune selection and renewal-safe retention of combo
  and legacy choices while PAYG/hourly references stay authoritative.
- `routing_deployment_test.go` and `routing_context_integration_test.go` host
  the end-to-end deployment/context regressions; no local suites were run.

## Static validation

`gofmt -l`, `go build ./...` and `go vet ./...` only. No local automated test
suite was executed; hosted CI remains the runtime gate.
