# Custom combo and core-change rewards

## Responsibility and data flow

Instant settlement and community raffle settlement both call
`applyDrawEntitlementTx`. Its core-change branch delegates to
`applyDrawCoreComboTx` in `activity_reward_combo.go` within the same transaction.
Provider updates continue through `xnode_sync_user` in the existing outbox.
No new external API or persistent tables are introduced.

A purchase with non-null `reward_renewal_price_minor` has custom terms, including
when that price is zero. Custom awards snapshot their selected core's reset
strategy into the existing `entitlement_reset_strategy` override. A core-change
prize preserves that override, materializing the current effective cadence first
for legacy custom purchases that still inherit it. Custom squads, price, traffic,
reset cadence, rollover and traffic reward lifetimes remain
intact. The current expiry is unchanged; subsequent renewals use the new base
combo's cadence. A later full custom prize replaces the custom terms normally.
An ordinary core change still adopts the target combo's default terms.
It clears a previous reset override so the target cadence actually takes effect.

## Existing affected purchases

The previous core-change handler set every custom override to NULL. Renewal
then inspected only the target base combo, which could have no included squads.
Fixing renewal validation alone could not restore the erased terms.

Migration `054_restore_custom_combo_after_core_reward.sql` uses immutable
`activity_draw_results` to recover current purchases with the exact erased-field
signature. It requires one unambiguous latest custom award within the purchase
window and a subsequent core-change reward matching the current base combo.
A single custom award and a core change may share a raffle settlement timestamp:
the final erased-field signature establishes that the core change ran last.

Recovery skips expired terms, overlapping purchase windows, existing renewal
successors, rollovers already being processed/settled, malformed reward payloads,
custom prizes tied at the latest timestamp, later traffic rewards, paid squad
additions, and audited purchase/user/database edits. These cases need individual
review through the existing administration tools; the migration does not infer
missing effect order or overwrite newer choices.

For eligible rows it restores custom squads, renewal price, current/renewal
traffic and rollover threshold; updates pending rollover metadata; records
`activity.custom_combo_recovered` with the source result ID; and queues one
existing provider-sync job per user. An already pending/processing identical job
is reused. The temporary candidate table is dropped within the migration.

The migration preserves the selected base combo, current expiry, purchase
charges, balance, ledger, coupons, historical draw outcomes and auto-renewal
switch. It neither repeats a draw nor automatically authorizes recurring charges.
Both the schema migration record and restored-field signature prevent reapplication.

## Core change followed by a custom combo

A later custom award writes its own squad, price, traffic and rollover overrides.
Those populated fields exclude the purchase from migration 054, including a
zero renewal price and awards sharing a settlement timestamp. If another custom
award existed before the core change, the latest-award guard also excludes that
older award. For separately timed rewards, a core change before the latest custom
award cannot authorize recovery even if the newer overrides are missing.

`activity_reward_combo_order_test.go` exercises these sequences through actual
reward settlement and compares all mutable entitlement fields before and after
the migration, alongside balances, recovery audit events and outbox job counts.

## Ordered recovery in migration 055

Migration 054 deliberately skipped multiple custom prizes sharing a timestamp
and any subsequent traffic reward. It also required all traffic overrides to be
NULL, so a traffic prize after the destructive core change prevented restoration.
Migration 055 rechecks current damaged terms, including cases missed when 054 ran.

Raffle settlement applies tickets in `created_at,id` order. Each settled ticket
links to its immutable result, so 055 uses that order within a single raffle and
compares result timestamps at full stored precision between settlements. Missing
or duplicate ticket links and tied timestamps across independent draws are not
ordered by guessing. A core change must follow the latest custom award and match
the purchase's current base combo. Intact or later custom awards remain untouched.

Recorded traffic rewards after the selected custom award are added to its current
allowance; only recurring rewards affect its renewal allowance. Before updating,
the migration verifies that surviving traffic overrides match the old behavior:
the current core allowance plus traffic awarded after the last core reset. It
rejects invalid resolved values, nonpositive/overflowed allowances, unexplained
traffic overrides, administrator entitlement/database edits, later paid squad
additions, overlapping purchase windows, successors and settled rollovers.
Unrelated user-level audit entries, such as a balance adjustment, do not prevent
recovery because no balance or financial record is changed.

Recovery uses the existing audit event and deduplicated provider-sync job with
`migration: 055_restore_ordered_custom_combo_rewards.sql`. Price, rollover and
squads come from the selected custom award; the new base combo and current expiry
remain unchanged. No stored result is rerolled and no renewal switch is enabled.

`activity_reward_ordered_recovery_test.go` settles real constructed raffles,
reproduces the former core-reset writes, and checks restoration and exclusions.
It checks current versus renewal traffic, newer-award protection, zero prices,
pending rollover metadata, unchanged balances and safe repeated execution.

## Reset cadence recovery in migration 056

The previous custom grant did not set `entitlement_reset_strategy`; core changes
therefore changed the effective cadence when that column was NULL. Migrations
054 and 055 restored other terms but did not fill this missing override.

Migration 056 finds the latest unambiguous custom award in settled ticket order,
matches its squads, renewal price and rollover threshold to a current custom
purchase, and fills only a NULL reset override. Automatic-renewal predecessor
references allow the current successor to retain the award's cadence without
rewriting expired terms. Explicit reset overrides, mismatched custom terms,
audited entitlement edits and rollovers already being processed remain untouched.
Recovery audit entries from 054/055 do not block this follow-up repair.

Legacy result payloads contain the selected core combo ID but no historical
cadence snapshot. The backfill therefore uses that original core's current
`reset_strategy`, including hidden cores; it cannot reconstruct an older setting
if the original core itself was edited. Future awards capture cadence at grant
time and subsequent core/catalog changes cannot silently replace it.

Each repair records `activity.custom_combo_cadence_recovered` with the source
award/core IDs and chosen strategy, and reuses or queues `xnode_sync_user`.
Traffic amounts, prices, balances, expiry, renewal switches and draw results do
not change. Provider sync applies the cadence without issuing a traffic reset.

## Validation and deployment

`activity_reward_combo_test.go` checks both reward orders, zero-price custom
terms, ordinary core behavior, and separate temporary/recurring traffic values.
`internal/catalog/automatic_renewal_reward_test.go` checks custom-then-core-change
enablement, charging and successor renewal. The migration tests check eligible
restoration, exclusions, unchanged balances/switches and safe reapplication.
Tests run in hosted CI; local automated tests are prohibited by `AGENTS.md`.

Rollover fixtures must create their row explicitly. Normal active purchases have
no rollover row until expiry is enqueued. The fixtures assert that snapshot
creation affects one row and use `MarkRolloverProcessing` for the processing
state, so a missing row cannot silently turn a protection test into an eligible
recovery case. Pending metadata refresh and processing-state preservation are
checked separately.

Deploy/restart the new image to apply migrations 054 through 056. Review
`activity.custom_combo_recovered` audit entries and the corresponding
`activity.custom_combo_cadence_recovered` entries and `xnode_sync_user` jobs,
then reopen the renewal dialog. CI success confirms the
constructed cases, not that a particular production purchase was repaired.
