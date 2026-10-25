# Embedded Xray diagnostics

`Quiet` installs the existing credential-safe discard policy once per process,
before either native configuration validation or connectivity probing builds an
embedded Xray configuration. Configuration builders log before the engine's
`none` logging setting applies; repeated startup validation otherwise floods
stderr with Shadowsocks/REALITY advisories and may expose credential-bearing
core diagnostics. Validation failures and safe probe outcomes still propagate
through their existing caller contracts. This does not change agent logging,
saved protocols, server ports or native validation rules.

`quiet.go` owns the shared one-time handler installation. `quiet_test.go` checks
concurrent initialization and proves raw credential diagnostics are not formatted.
