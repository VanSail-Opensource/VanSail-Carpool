# WT-HK Agent Upgrade Outage — 2026-10-18

## Executive summary

WT-HK (`75f85947-99fd-435b-8307-95c6b7c9fdab`) stopped serving after a
`v2.0.5 → v2.0.7` agent upgrade and the automatic rollback that followed.
Two independent faults combined:

1. **Rollback/watchdog deadlock.** The transient
   `x-node-upgrade-rollback.service` held `/var/lib/x-node/updates/lock`
   across a blocking `systemctl restart x-node.service`. The restarted agent
   called `update.Manager.Confirm`, blocked on the same `flock`, and kept the
   update mutex locked. The agent main loop then blocked in
   `Manager.Pending()` and stopped feeding the systemd watchdog, so systemd
   aborted it every 60 seconds and restarted it. The rollback unit stayed
   active for ~40 minutes.
2. **Native config validation without the asset path.** After the rollback,
   the node ran `v2.0.5` and kept failing activation. `Agent.activate` calls
   `xray.Prepare` **before** injecting
   `env.XRAY_LOCATION_ASSET`; the compiled config contains
   `geosite:category-ai-!cn`, so Xray tried to open
   `<asset-path>/geosite.dat`, found nothing, and rejected the config. The
   service had no process-level `XRAY_LOCATION_ASSET`. The failure happened
   before `Runtime.Apply`, so no listener was ever created.

Service was restored at **20:05 CST** by releasing the stuck lock and
providing a stable process-level asset path. The agent is now
`ready=true`, `synchronized=true`, at authorization/config revision `98`, and
listening on `1145` and `8443`.

## Impact

- WT-HK unavailable from approximately **18:38 to 20:05 CST** (~1 h 27 min).
- The panel showed `connected=true` but `ready=false`, so the node appeared
  online while carrying no traffic.
- No configuration, accounting, or credential data was lost or rewritten.
  `state.json` and `jobs.json` remained valid throughout.
- Both installed binaries were verified against the signed `v2.0.5`
  `SHA256SUMS` manifest; the rollback did not corrupt them.

## Timeline (CST, 2026-10-18)

| Time | Event |
| --- | --- |
| 18:11 | `update.install` (`v2.0.7`) queued. The agent downloaded and staged the release, then reported `systemctl failed` because the restart command is a child of the unit it restarts. Inline rollback ran. |
| 18:12–18:13 | Additional update checks/installs and transient rollback timers. |
| 18:32–18:35 | `v2.0.7` running; Xray started and the service reached `READY=1`. The rollback timer was still armed because `Confirm` did not complete. |
| 18:37:47 | Agent stopped and restarted by the update flow. |
| 18:38:32 | Rollback timer fired, restored `v2.0.5`, and started `systemctl restart` inside `-upgrade-rollback.service`. |
| 18:38–19:18 | Rollback helper held the update `flock` while waiting for the restart. New agent blocked in `Confirm`; main loop blocked in `Pending`; watchdog aborts repeated. |
| 18:55 | Last durable `state.json` write (authorization revision 98, config revision 88). No activation past `Store.Accept` afterwards. |
| 19:18 | Stuck transient rollback unit/timer stopped, releasing the lock. Activation still failed. |
| 19:21–19:59 | Repeated restarts; `ready=false`; no Xray listeners; reload commands remained queued. |
| 19:59 | Native validation failure reproduced locally with the pinned X-Node source. Exact error: `failed to open geosite.dat`. |
| 20:04 | Agent run manually with `XRAY_LOCATION_ASSET` set: `ready=true`, config revision 98, listeners bound. |
| 20:05 | Systemd drop-in installed and service restarted. |

## Evidence

### Agent state before recovery

```
/var/lib/x-node/status.json
{"config_revision":88,"connected":true,"liveness":true,"ready":false,
 "revision":98,"updated":"2026-10-18T11:20:44Z"}
```

Systemd reported the same condition:

```
StatusText="Runtime ready=false synchronized=false connected=true
            authorization=98 configuration=88"
ActiveState=activating
```

No `ss -lntp` entries existed for `1145` or `8443`.

### Deadlock

- `/var/lib/x-node/updates/transaction.json`:
  `{"version":"v2.0.7","previous":"v2.0.5","status":"rolled_back",...}`
- `/proc/locks` contained a `FLOCK` on `/var/lib/x-node/updates/lock` owned by
  the `xnodectl.old upgrade-rollback` PID. The same PID had
  `systemctl restart x-node.service` as a child.
- A `SIGQUIT` goroutine dump of the agent showed:
  - `update.(*Manager).Pending` blocked on `sync.Mutex`;
  - `update.(*Manager).Confirm` blocked in `host.Lock` →
    `golang.org/x/sys/unix.Flock`.
- The main loop therefore never reached the watchdog tick, producing
  `Failed with result 'watchdog'` every 60 seconds.

### Native validation

The compiled Xray document in `state.json` contains:

```json
"routing": {"domainStrategy":"AsIs","rules":[
  {"domain":["geosite:category-ai-!cn"],"outboundTag":"HNTW"}]}
```

and has no `env` object. Running the pinned `v2.0.5` `xray.Prepare` on the
exact stored config produced:

```
Xray configuration validation failed:
  ... common/geodata: illegal domain rule: geosite:category-ai-!cn
  ... common/geodata: failed to open geosite.dat
```

Adding `env.XRAY_LOCATION_ASSET` to the document (or exporting it in the
process environment) made the same config validate and start successfully.

## Root cause 1 — update lock held across restart

Relevant upstream code (`internal/update/manager.go`):

- `Install` takes `m.mu` and `host.Lock(...)` and keeps both while calling
  `systemctl restart x-node.service` synchronously.
- `Confirm` takes `m.mu`, then `host.Lock(...)`.
- `Rollback`/the `upgrade-rollback` CLI takes the same `host.Lock` and keeps
  it while calling `systemctl restart` and waiting for the unit.
- `Pending` takes `m.mu` and is called from the agent hello/status path.

Because the rollback helper is a separate process, the restarted agent cannot
acquire the lock, and the helper cannot finish until the restart succeeds — a
cross-process deadlock. `Pending` blocking on the same mutex starves
`WATCHDOG=1`, so systemd kills the agent. The restart is also issued from
inside the unit being restarted, so `systemctl` frequently reports failure
even when the restart proceeds.

## Root cause 2 — validation before asset-path injection

`internal/agent/agent.go` (`activate`) currently does:

```go
resolved, e := a.Certificates.Prepare(lifetime, c)
...
if _, e = xray.Prepare(resolved); e != nil {
    return e
}
...
// asset path is injected only here, after validation
env["XRAY_LOCATION_ASSET"] = geodata.Resolve(filepath.Join(a.Dir, "geodata"))
...
a.Runtime.Apply(lifetime, resolved)
```

`xray.Prepare` loads routing rules at configuration-parse time. For
`geosite:`/`geoip:` matchers Xray opens `geosite.dat`/`geoip.dat` from
`XRAY_LOCATION_ASSET` (falling back to the executable directory). The
document-level `env` is honored, but it is not present in the first
validation call. The installed unit provides no process-level
`XRAY_LOCATION_ASSET`, so this makes **every** activation fail, before the
runtime is touched. `v2.0.7` has the same ordering, so this is not specific
to the rollback target.

> Open question: systemd does not record the environment of the earlier
> serving process, so the exact reason the pre-upgrade process validated
> successfully could not be reconstructed from retained logs. The durable
> fix therefore removes the dependency on ambient environment entirely.

## Mitigation applied

1. Stopped the stuck transient rollback unit and timer, releasing the lock:

   ```bash
   systemctl stop x-node-upgrade-rollback.timer \
                  x-node-upgrade-rollback.service
   ```

2. Added a stable asset directory and systemd drop-in:

   ```bash
   # /usr/local/sbin/x-node-asset-link.sh
   #!/bin/sh
   set -eu
   active=/var/lib/x-node/geodata/active.json
   [ -r "$active" ] || exit 0
   gen=$(sed -n 's/.*"generation":"\([0-9a-f]\{64\}\)".*/\1/p' "$active")
   [ -n "$gen" ] || exit 0
   ln -sfn "/var/lib/x-node/geodata/$gen" /var/lib/x-node/geodata/current
   ```

   ```ini
   # /etc/systemd/system/x-node.service.d/asset-path.conf
   [Service]
   Environment=XRAY_LOCATION_ASSET=/var/lib/x-node/geodata/current
   ExecStartPre=/usr/local/sbin/x-node-asset-link.sh
   ```

3. `systemctl daemon-reload`, then stopped/started the service with
   `--no-block` so the command cannot hang on readiness.

4. Verified:

   ```
   ready=true synchronized=true connected=true
   authorization=98 configuration=98
   LISTEN *:1145 (x-node)   LISTEN *:8443 (x-node)
   ```

   Panel metrics for WT-HK now report `ready=true`, `synchronized=true`,
   `version=v2.0.5`, `xrayVersion=v26.10.10`.

5. Confirmed no `x-node-*` timers remain and the update transaction is
   `rolled_back`. Diagnostic traces and temporary files were removed.

No database edits were required.

## Suggested code edits

### X-Node (upstream, required for a durable fix)

#### 1. Inject the asset path before native validation

`internal/agent/agent.go` — move the `env` injection above `xray.Prepare`
and set the process environment as a fallback:

```go
resolved, e := a.Certificates.Prepare(lifetime, c)
if e != nil { return e }

assetDir := geodata.Resolve(filepath.Join(a.Dir, "geodata"))
_ = os.Setenv("XRAY_LOCATION_ASSET", assetDir) // process-wide fallback

var native map[string]any
_ = json.Unmarshal(resolved.Xray, &native)
env, _ := native["env"].(map[string]any)
if env == nil { env = map[string]any{} }
env["XRAY_LOCATION_ASSET"] = assetDir
native["env"] = env
resolved.Xray, _ = json.Marshal(native)

if _, e = xray.Prepare(resolved); e != nil { return e }
```

Setting the environment once in `Agent.Run` (and updating it when the active
generation changes) is also acceptable and protects any future pre-validation
path.

#### 2. Never hold the update lock across a restart

`internal/update/manager.go`:

- Split `rollback` into a locked state mutation (restore `.old`, write
  `rolled_back`) and an unlocked restart:
  ```go
  // under lock: restore binaries, write transaction
  // after unlock:
  if _, e := m.Runner.Run(ctx, "systemctl", "restart", "--no-block",
                            "x-node.service"); e != nil { return e }
  ```
- Do the same in `Install` (stage, rename, write `pending`, then release the
  lock before `systemctl restart --no-block`).
- Stop the transient timer before restarting:
  `systemctl stop x-node-upgrade-rollback.timer`.
- Do not treat `systemctl` exit status as activation success. Confirm the
  upgrade via the agent's `state.applied` / a readiness probe instead.
- Make `upgrade-rollback` idempotent and ensure it exits after restoring
  binaries; it must not wait for the agent while holding the lock.

#### 3. Never block the watchdog path on the update mutex

`internal/update/manager.go` / `compatibility.go`:

- `Pending()` must not take the same mutex as `Confirm`/`Install` while they
  wait on `flock`. Use a dedicated `sync.RWMutex` for the in-memory status,
  cache the last transaction status atomically, or use `TryLock` with a
  bounded fallback.
- `Confirm` should acquire the host lock with `LOCK_NB` plus a short retry
  budget and return a recoverable error instead of blocking indefinitely.

#### 4. Make validation errors actionable

`internal/kernel/xray/config.go`:

```go
pb, err := confserial.LoadJSONConfig(bytes.NewReader(b))
if err != nil {
    return nil, fmt.Errorf("Xray configuration validation failed: %w", err)
}
```

This turns the opaque `Xray configuration validation failed` into the exact
missing-asset or invalid-rule diagnosis.

### Panel (this repository)

#### 1. Surface `state.failed` errors (memory only)

`internal/xnode/master_receive.go` currently drops the agent's activation
error. Retain a bounded, redacted message on the live snapshot and expose it
in node metrics/logs so a failure like this is immediately visible:

```go
case "state.failed", "error":
    // existing command/superseded handling...
    if message.Type == "state.failed" {
        var body map[string]string
        if wire.Decode(message.Data, &body) == nil {
            m.mu.Lock()
            snapshot := m.live[peer.nodeID]
            // The agent already redacts secrets; still bound the length and
            // keep this memory-only.
            snapshot.activationError = boundedText(body["error"], 512)
            m.live[peer.nodeID] = snapshot
            m.mu.Unlock()
        }
    }
```

Add `activationError` to `liveSnapshot` (`metrics_types.go`) and `NodeMetrics`
(`master_metrics.go`). Do not persist it.

#### 2. Bound command delivery and report undelivered work

`internal/xnode/operations.go` (`runProviderOperation`): when the node is
connected but not `ready`/`synchronized`, fail `runtime.reload`/`update.*`
immediately with a durable `XNODE_NODE_NOT_READY` error instead of leaving
receipts `queued`. Add a delivery deadline (for example 15–30 s) after which
the operation transitions to a terminal failure with
`XNODE_COMMAND_UNDELIVERED`. During this incident three reload receipts
stayed `queued` for minutes and later completed; one remained queued after
recovery.

#### 3. Guard `update.install`

Before accepting `update.install`, preflight both the target **and** the
rollback target against the node's current compiled configuration and
capabilities (using the existing `VerifyRelease` /
`configurationCapabilities` paths). Refuse the install when:

- a transaction is already `pending`; or
- neither the current nor the previous binary can validate the active config
  (for example, a geosite rule without an asset path).

This prevents a rollback from downgrading into a configuration the older
agent cannot activate.

## Preventive recommendations

- Keep `asset-path.conf` and the `current` symlink updater until an X-Node
  release includes the reordered asset injection.
- Do not trigger another `update.install` on WT-HK until the updater lock and
  restart fixes are available. `config.updates.enabled` is already `false`
  on this node.
- Treat "ready and synchronized with listeners bound" as the rollback success
  criterion; do not rely on `systemctl restart` exit status.
- Add a post-upgrade smoke check that verifies
  `status.ready && status.synchronized &&
   state.config_revision == state.revision` and that the configured inbound
  ports are listening.
- Prefer `systemctl restart --no-block` plus an out-of-band readiness probe
  for any restart initiated from inside `x-node.service`.
- Consider a host-level unit ordering rule that prevents
  `x-node-upgrade-rollback.service` from overlapping a running agent
  activation.

## Appendix — diagnostic commands used

```bash
systemctl status x-node.service x-node-upgrade-rollback.service
journalctl -u x-node.service --since '...' -o cat
cat /var/lib/x-node/status.json
cat /var/lib/x-node/updates/transaction.json
cat /proc/locks
kill -QUIT <agent-pid>            # goroutine dump, service auto-restarts
strace -f -e trace=openat,connect,bind -p <agent-pid>
XRAY_LOCATION_ASSET=/var/lib/x-node/geodata/<generation> \
  /usr/local/bin/x-node --config /etc/x-node/bootstrap.json \
  -state-dir /var/lib/x-node
```

The native-validation reproduction used the pinned X-Node source
`github.com/VanSail-Opensource/X-Node@5352b67e882b` (v2.0.5) and the exact
`state.json` configuration plus users.
