# Frontend maintenance scripts

- `generate-response-schemas.mjs` resolves OpenAPI response references and generates structural read contracts without server-owned value restrictions. It verifies Zod can compile every generated schema.

- `compact-contract.mjs` keeps the generated OpenAPI type artifact below the
  repository line ceiling without changing its TypeScript declarations.
- `check-structure.mjs` audits source line limits, folder documentation,
  locale parity, script/template copy, local icon-asset policy, and coverage
  of production-source icons by the Vite client bundle scan or explicit list.

`generate-diy-contracts.mjs` derives Go pointer types, field metadata, API override schemas and bilingual labels from the canonical subscription allowlist. Regenerate these before `generate:api` when changing the allowlist.

Catalog ownership generates separate member `ConnectionOverrides` and
administrator `EndpointOptions` Go/API types. Existing member labels and retired
keys remain confined to member/shared fields; administrator labels belong to the
endpoint editor's localization domain. Inclusive port ranges and field-specific
range bounds are generated for administrator controls without a raw JSON editor.

`workflow-policy.mjs` exports the pure `auditWorkflow(name, parsedYaml)` security check for full upstream SHAs, approved repositories, private checkout credentials and bounded job permissions. `check-workflows.mjs` reads `.github/workflows` through the existing YAML dependency and reports static failures without network access or executing jobs. `workflow-policy.regressions.mjs` supplies hostile-ref, privilege and publication-bypass regressions through Node's built-in runner in hosted Actions only. `audit:structure` includes this static gate; `test:workflows` is the separate hosted regression entry point. Extend the reviewed action/job allowlists deliberately and update `docs/actions-security.md` when changing pins.
