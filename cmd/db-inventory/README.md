# Schema inventory command

`main.go` creates a private up-to-date SQLite fixture and completes the real
identity migration before exporting schema and synthetic bootstrap rows. An
explicit `--source` instead opens an existing source read-only and exports only
metadata/counts. `main_test.go` verifies deterministic independent fixtures and
the post-startup schema in hosted CI. See the dbinventory package for contracts.
