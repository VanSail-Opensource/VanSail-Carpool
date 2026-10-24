# Database conversion command

`main.go` exposes `inspect`, `export`, `import`, `verify`, and `cutover-check`.
Database URLs are read from protected runtime configuration, never command-line
arguments. `--source` is read-only. `export --snapshot` and `--output` refuse existing
destinations and write mode 0600. No HTTP server, upstream executors, scheduler or
X-Node master starts. See datamigrate for source/schema/data/recovery gates.
