# Pinned X-Node compute protocol

Source: https://github.com/VanSail-Opensource/X-Node/tree/c310c8f6cfcce9d6081a5593c7232f1e1c3e550a
Signed agent release: v2.0.15. The panel Go protocol/native/compute dependency
uses that exact source revision. `compute-pool.md` is its protocol reference;
public Go message contracts are in the pinned `pkg/protocol` and `pkg/compute`.
Older agents remain supported but cannot claim compute batches.

Quality run 40366302761 and signed release run 40369427102 completed successfully.
The downloaded SHA256SUMS signature was verified with the repository trust key;
the trusted version comment equals v2.0.15. All six manifest entries were checked.
The amd64 agent binary records this exact revision with vcs.modified=false.

- amd64 agent SHA-256: 2fce34e0a44e0be7068c175e66649c9d632032924b02a0416d93357efa01879e
- arm64 agent SHA-256: a9678c312fc5692dca2baac633e77e91646822af5f65c4b775e8f222c6d3c66e

Artifact verification is separate from executing local tests. Hosted agent
regressions and real-host checks passed before the release tag was published.
