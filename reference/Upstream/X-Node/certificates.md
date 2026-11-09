# Certificate lifecycle

Agents advertise `certificate_lifecycle_v1`. A supporting master submits
`certificates.run` through its existing durable per-node command queue, with
`profile_id`, `action` (`issue` or `renew`) and positive `expected_revision`.
Only ACME HTTP/DNS profiles accept these actions. File/content replacement uses
configuration validation/reload and self-signed profiles retain normal activation.
Execution is bounded to five minutes; delivery has its separate existing bound.

The actor captures the latest desired configuration, including an unactivated
profile after an earlier failure. Accepted configuration can describe an older
profile and is not used to select the requested target. New attempts check the
desired revision before acquisition and again after waiting for profile locks.
The actor rechecks profile identity/revision before scheduling activation, and
the native worker pins and verifies the acquired fingerprint before applying it.
Supersession prevents obsolete material from beginning activation.

`issue` reuses existing valid material. `renew` explicitly requests renewal even
outside the normal renewal window. CertMagic's public synchronous APIs perform
the attempt under its shared per-domain storage coordination; its public
PreChecker extension disables interactive prompts while email remains optional.
Background management uses agent lifetime rather than command timeout. ACME
failure leaves previously valid in-memory and serving material available.

`command.progress` uses the command ID in its envelope and typed data
`{profile_id, action, phase}`. Phases are `issuing`, `renewing`, `applying`, `idle`
and `failed`. Profiles also appear in telemetry and `certificates.get`, including
failed desired profiles. Public status includes mode/domain, serving fingerprint,
validity/SANs/issuer, acquired fingerprint, current/last command ID, safe error
code and attempt/success timestamps. `revision` is a decimal string. Status reads
never wait for ACME, disk cleanup or per-profile operation locks.

Serving evidence comes from the exact immutable prepared material immediately
after native runtime success. It does not follow a manager's later renewal.
`active` and `fingerprint` therefore remain truthful when firewall/state
acceptance fails after the replacement begins serving. `acquired_fingerprint`
can differ while a newer certificate awaits activation. A successful result
records `{profile_id, action, fingerprint, expires, applied, ready}`; an uncertain
result retains only those public facts when available.

The existing durable job intent/result journal owns replay. Same-ID retries
return retained receipts before checking current profile/revision eligibility;
an uncertain attempt is never reissued automatically, including after restart.
The master must keep the original command ID and agent identity while reconciling
uncertainty. A replacement agent does not prove the prior issuance outcome.

No certificate PEM, private key, provider credentials, raw provider errors or
node-local material paths enter lifecycle status, progress or durable results.
Profiles are bounded to 64 per desired configuration. SAN observations are
bounded to 128 names and issuer text to 512 bytes. Files
remain in private content-addressed cache directories, with current/previous
runtime generations retained through cleanup and rollback.
