# Durable job recovery and routing evidence

Account purge owns one outbox job per account generation. Scheduling locks the
account row and resumes an exhausted job instead of adding another operation.
`outbox.WaitError` preserves attempts and the provider/dependency delay;
`ReviewError` and `TerminalError` stop automatic delivery. Telegram blocked,
deactivated, and missing private recipients are terminal notification failures;
429 honors `retry_after`, while transient provider failures retain bounded retries.
This delivery classification never authorizes account deletion.

Protected-address bank delivery remains independent of ordinary firewall/node
enablement: connected agents must acknowledge DDNS bypass staging even when
ban enforcement is disabled. Only retired nodes supersede that protection work.

Native period closure registers admission under the period row lock before
sending a grant. Closing fences later registration, then synchronizes and seals
only recorded participants. Offline participants retain unfinished accounting.
Purge and resumed settlement do not hold the shared service mutation mutex
during network waits; closing-period scans rotate past blocked older batches.
Final closure cannot overwrite a reopened period or a newer closure boundary.

Routing preflight and deployment inspect the effective native projection for
the selected source principal/listener and the target transit generation. The
deployment check uses the immutable prepared configuration whose digest was
sent, confirms the current peer's applied revision, and rechecks plan ownership,
revision, and phase. Revocation checks include previous choices and owned transit
credentials. These checks require no agent protocol extension.

Routing endpoint projections include `billingMode`; localized member selectors
show billing mode plus address/port. A changed dependency digest produces a
review reason even while the durable reconciler still records the old active
phase. A successful old receipt is not proof of a current routing generation.

## Incident baseline and verification

Read-only production investigation on 2026-11-08 found image revision `f642e76`,
59 failed purge jobs, and failed offline/firewall and access-repair attempts.
User 8485399326 had active routing revision 22, with applied participating-node
revisions. No failing destination/client identity was supplied, so a source
credential mismatch remains unproven. Do not resave their plan or clear failed
jobs merely to replace this evidence.

Hosted regressions cover admission fencing, settlement scan fairness, generation
recovery, review stops, notification classification and stale routing UI. Local
validation uses static Go/TypeScript checks and Chrome DevTools fixtures; local
automated suites remain prohibited. Production acceptance must compare an actual
source identity and matching destination after the verified image is deployed.
