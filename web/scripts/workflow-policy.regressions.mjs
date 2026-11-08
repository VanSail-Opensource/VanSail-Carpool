import assert from 'node:assert/strict'
import test from 'node:test'
import { auditWorkflow } from './workflow-policy.mjs'

const sha = 'a'.repeat(40)
const checkout = () => ({ uses: `actions/checkout@${sha}`, with: { 'persist-credentials': false } })
const workflow = () => ({ on: { push: {} }, permissions: { contents: 'read' }, jobs: { build: {
  needs: 'quality', permissions: { contents: 'read', packages: 'write' }, steps: [checkout()],
} } })

test('approved SHA references preserve read-only quality and bounded publication', () => {
  assert.deepEqual(auditWorkflow('container.yml', workflow()), [])
  const value = workflow()
  value.jobs.build.steps.push({ uses: './.github/actions/local' })
  assert.deepEqual(auditWorkflow('container.yml', value), [])
})

test('mutable refs, short SHAs and unreviewed repositories fail closed', () => {
  for (const ref of ['actions/checkout@v7', 'actions/checkout@abcdef0', `unknown/publisher@${sha}`]) {
    const value = workflow()
    value.jobs.build.steps.push({ uses: ref })
    assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('full commit SHA')))
  }
})

test('default writes and writes outside the designated job are rejected', () => {
  const value = workflow()
  value.permissions.contents = 'write'
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.startsWith('container.yml: unapproved')))
  value.permissions = 'write-all'
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('mapping required')))
  assert.ok(auditWorkflow('ci.yml', workflow()).some(issue => issue.includes('packages permission')))
})

test('persisted checkout credentials and publication bypasses are rejected', () => {
  const value = workflow()
  delete value.jobs.build.steps[0].with
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('persist credentials')))
  value.jobs.build.if = 'always()'
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('successful quality')))
  delete value.jobs.build.needs
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('successful quality')))
  value.on.pull_request_target = {}
  assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('cannot publish')))
  for (const event of ['pull_request', ['push', 'pull_request_target']]) {
    value.on = event
    assert.ok(auditWorkflow('container.yml', value).some(issue => issue.includes('cannot publish')))
  }
})
