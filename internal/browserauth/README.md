# Browser administrator access

`New` receives existing account storage, the public URL, and the deployment
administrator allowlist. Access comes from five-minute one-use Mini App links.
`service.go` owns link issuance, atomic exchange, role/allowlist verification,
independent device-session restoration, renewal, and logout.

Only SHA-256 token references are persisted. Browser sessions expire after thirty
inactive days and renew at most daily without changing the bearer or request key.
The member Mini App session remains independent. Every browser request rechecks
administrator eligibility; logout revokes only that browser device.

`service_test.go` covers handoff authorization, destination confinement and
allowlist changes. HTTP integration scenarios live beside the transport.

Exchange returns the user read and authorized while the one-use handoff is
locked in the same database transaction. The current role, deployment allowlist,
account availability, and destination are checked before creating the session.
Storage failures retain their error identity and roll back the link consumption;
only absent, expired, or unauthorized handoffs become `ErrUnauthorized`.
`exchange_recovery_test.go` covers owner-read/session-insert failures, retry of
the same link after repair, and authorization changes before exchange.
