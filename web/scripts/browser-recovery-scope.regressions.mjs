import assert from 'node:assert/strict'
import test from 'node:test'
import { existsSync } from 'node:fs'
import { allBrowserScopes, memberBrowserScopes, adminBrowserScopes, browserRecoverySuites, parseBrowserScopes } from './browser-recovery-scopes.mjs'
import { selectBrowserRecovery } from './browser-recovery-selection.mjs'

test('focused PRs select related suites without unrelated administrator recovery', () => {
  assert.deepEqual(selectBrowserRecovery(['web/src/components/hourly/HourlyAllowance.vue']).scopes, ['hourly', 'settings'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/admin/database/DatabaseRecordEditor.vue']).scopes, ['admin-database'])
  assert.deepEqual(selectBrowserRecovery(['web/scripts/browser-xnode-creation.mjs']).scopes, ['xnode-creation'])
  assert.deepEqual(selectBrowserRecovery(['web/scripts/browser-hourly-preferences.mjs']).scopes, ['hourly'])
  assert.deepEqual(selectBrowserRecovery(['web/scripts/browser-member-design-checks.mjs']).scopes, ['member-design'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/activity/BetGamesPanel.vue']).scopes, ['activity', 'member-design'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/subscription/HostLinkRow.vue']).scopes, ['member', 'member-design'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/hourly/HourlyAllowanceGraph.vue']).scopes, ['hourly', 'settings', 'member-design'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/hourly/hourlyAllowance.ts']).scopes, ['hourly', 'settings', 'member-design'])
  assert.deepEqual(selectBrowserRecovery(['web/src/components/catalog/BillingDemo.vue']).scopes, ['demonstrations'])
  assert.deepEqual(selectBrowserRecovery(['web/fixtures/member-utility-data.ts']).scopes, ['member-design'])
})

test('member theme and browser theme have independent recovery scope', () => {
  assert.deepEqual(selectBrowserRecovery(['web/src/styles/member-design.css']).scopes, memberBrowserScopes)
  assert.deepEqual(selectBrowserRecovery(['web/src/browser/styles.css']).scopes, allBrowserScopes.filter(scope => adminBrowserScopes.includes(scope)))
  assert.deepEqual(selectBrowserRecovery(['web/src/styles/fonts.css']).scopes, allBrowserScopes)
  assert.deepEqual(selectBrowserRecovery(['web/public/fonts/SpaceGrotesk-variable.woff']).scopes, memberBrowserScopes)
})

test('documentation, unit-only and backend-only PRs need no browser job', () => {
  const paths = ['AGENTS.md', 'docs/agent-tooling.md', 'web/src/components/hourly/HourlyPage.test.ts', 'web/scripts/browser-recovery-scope.regressions.mjs', 'internal/hourly/service.go']
  assert.deepEqual(selectBrowserRecovery(paths).scopes, [])
})

test('shared runtime, configuration and unmapped frontend changes keep full coverage', () => {
  for (const path of ['web/package-lock.json', 'api/openapi.yaml', '.github/workflows/ci.yml', 'web/src/utils/request.ts', 'web/src/new-feature/entry.ts']) {
    assert.deepEqual(selectBrowserRecovery([path]).scopes, allBrowserScopes, path)
  }
})

test('deleted or moved paths retain both affected areas and stable duplicate-free ordering', () => {
  const plan = selectBrowserRecovery(['web/src/components/hourly/Old.vue', 'web/src/components/admin/database/New.vue', 'web/src/components/hourly/Old.vue'])
  assert.deepEqual(plan.scopes, ['hourly', 'settings', 'admin-database'])
  assert.equal(plan.reasons.length, 2)
  assert.throws(() => selectBrowserRecovery(['../outside.vue']), /Invalid changed path/)
  assert.throws(() => selectBrowserRecovery(['/tmp/a.vue']), /Invalid changed path/)
})

test('suite scope is explicit, non-empty, known and safe from inherited object keys', () => {
  for (const raw of [undefined, '', 'null', '[]', '{}', '["all"]', '["toString"]', '["__proto__"]', '[1]', '[null]']) {
    assert.throws(() => parseBrowserScopes(raw))
  }
  assert.deepEqual(parseBrowserScopes('["hourly","settings","hourly"]'), ['hourly', 'settings'])
  for (const suite of Object.values(browserRecoverySuites)) assert.ok(existsSync(new globalThis.URL(suite.module, import.meta.url)))
})
