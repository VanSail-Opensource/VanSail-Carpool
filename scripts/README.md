# Repository maintenance scripts

`generate_db_retries.go` regenerates PostgreSQL complete-operation retry adapters
for reviewed Store methods named `retryAttempt_...`. Run it from the repository
root with `go run scripts/generate_db_retries.go`; commit the generated adapters
and the database README map together. CI requires regeneration to leave no diff
or untracked database files.

Opting a method in requires reviewing the whole attempt: one database transaction,
all writes rolled back on failure, and no provider calls or non-replayable local
side effects. The generator does not add retries to arbitrary methods, rename
production functions, or change their locking policy. Methods with explicit
manual retry loops retain that control. PostgreSQL is the only supported runtime;
the retired compatibility-database lock guard must not be generated.

`ci/` contains hosted change detection, race-test inventory/sharding, production
image and PostgreSQL recovery verification. Its README describes those gates.
