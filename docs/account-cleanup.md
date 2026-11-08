# Evidence-based account cleanup

Ordinary-account reachability checks use numeric private-chat IDs and the
existing Telegram queue. `sendChatAction` reports whether the typing action was
accepted; `getChat` provides current chat metadata. These methods establish
reachability, not definitive account existence. The method contracts and error
envelope are documented by the [Telegram Bot API](https://core.telegram.org/bots/api).
Error descriptions can change, so unrecognized results remain unknown.

The scheduler first commits an exact account generation, bot epoch and nonce
with its existing-outbox job. The worker reloads that authorization before making
either call. One process-wide nonblocking gate limits probes; each method has
two seconds and the whole turn has seven. Other ordinary outbox and Telegram
workers remain available. No separate external executor is introduced.

The paired classifier accepts only exact typed HTTP/API errors for the requested
method. Timeouts, throttling, server errors, authentication/configuration errors,
unexpected IDs/types and contradictory results cannot authorize account removal.
A successful action cancels failure evidence. Generic unreachable responses
require earlier strong private reachability in the same verified bot epoch.

The repository requires three qualifying observations separated by at least
24 hours, spanning seven days, followed by a separate fresh final probe. New
authenticated activity or unblock evidence cancels the candidate. Correlated
failures pause admission while recovery probes continue. Configured and stored
administrators are protected at scheduling, completion, admission and final purge.

Bot verification pauses admission before queued `getMe`; the returned bot ID
must match the current configured token identity. Failure leaves cleanup paused.
A different verified bot advances the epoch and invalidates old evidence.
Verification refreshes during existing Telegram setup, before the 15-minute TTL.

Only the established generation-fenced purge lifecycle removes operational data.
Immutable financial/accounting evidence and replay tombstones remain retained;
uncertain native accounting stays pending with its actual dependency. Cleanup
never clears a queue or counter to manufacture a successful state.

Proven unattempted intents are superseded atomically at admission. Recorded
attempts, legacy Emby create markers and live checkouts retain their exact
continuations; elapsed maintenance age cannot authorize a refund or erase an
unknown outcome. A fresh final reachability check does not release an explicit
operator-review hold. Each destructive phase rechecks current protection and
evidence, including a database-clock refresh before final wallet/data removal.

Native period and scoped-IP settlement uses fair cursors in the same durable
purge job. Old firewall allowances are revoked through the existing fleet
revision queue before deletion; sanitized purchase receipts remain in audit.
Re-enabling a host forces a full firewall snapshot and a new receipt epoch.
Offline participants and opaque restriction outcomes remain specific pending
dependencies. Agent-local presentation IP history can last for its bounded
history window; immediate erasure of that memory is not asserted. It cannot
restore the removed credentials, paid access or byte-credit authorization.
