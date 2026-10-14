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
