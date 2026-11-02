# Hosted PostgreSQL fixtures

`Open` creates an isolated database for one test or benchmark. It clones the
fully migrated `POSTGRES_CONTRACT_URL` template, or initializes a process-private
template through `POSTGRES_TEST_ADMIN_URL` / `POSTGRES_TEST_URL`. Credentials
must have database creation permission. Cleanup closes connections and removes
each fixture; process-private templates disappear with the hosted service.

Fixtures use the production PostgreSQL connector and schema. They do not emulate
another database engine or run application workers. No local test runner is
required or authorized by this helper.

`database.go` clones isolated PostgreSQL fixtures and registers bounded cleanup. `template.go` prepares one migrated template per hosted test process; POSTGRES_FIXTURE_URL supplies private administrator credentials. Templates are disposed with the hosted database service.
