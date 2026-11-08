const approvedActions = new Set([
  'actions/checkout', 'actions/setup-node', 'actions/setup-go',
  'actions/upload-artifact', 'actions/download-artifact', 'actions/attest-build-provenance',
  'docker/setup-buildx-action', 'docker/login-action', 'docker/metadata-action',
  'docker/build-push-action', 'dorny/paths-filter',
])

const writes = {
  'container.yml:build': new Set(['packages']),
  'container.yml:publish': new Set(['packages', 'id-token', 'attestations']),
  'dependabot-auto-merge.yml:auto-merge': new Set(['contents', 'pull-requests']),
}

// Validate workflow data without executing repository code or contacting GitHub.
export function auditWorkflow(name, workflow) {
  const failures = []
  function permissions(value, allowed, scope) {
    if (!value || typeof value !== 'object' || Array.isArray(value)) {
      failures.push(`${scope}: explicit permission mapping required`)
      return
    }
    for (const [key, access] of Object.entries(value)) {
      if (!['read', 'none', 'write'].includes(access) || (access === 'write' && !allowed.has(key))) {
        failures.push(`${scope}: unapproved ${key} permission`)
      }
    }
  }
  function action(ref, scope) {
    if (typeof ref !== 'string') return
    if (ref.startsWith('./')) return
    const match = /^([^@]+)@([0-9a-f]{40})$/.exec(ref)
    if (!match || !approvedActions.has(match[1])) failures.push(`${scope}: reviewed action with full commit SHA required`)
  }
  permissions(workflow.permissions, new Set(), name)
  if (!workflow.jobs || typeof workflow.jobs !== 'object') failures.push(`${name}: jobs required`)
  for (const [id, job] of Object.entries(workflow.jobs ?? {})) {
    const scope = `${name}:${id}`
    permissions(job.permissions ?? workflow.permissions, writes[scope] ?? new Set(), scope)
    action(job.uses, scope)
    for (const step of job.steps ?? []) {
      action(step.uses, scope)
      if (step.uses?.startsWith('actions/checkout@') && step.with?.['persist-credentials'] !== false) {
        failures.push(`${scope}: checkout must not persist credentials`)
      }
    }
    if (name === 'container.yml' && writes[scope]) {
      const needs = Array.isArray(job.needs) ? job.needs : [job.needs]
      if (!needs.includes('quality') || (job.if !== undefined && job.if !== 'success()')) {
        failures.push(`${scope}: publication must require successful quality`)
      }
      const events = typeof workflow.on === 'string' ? [workflow.on]
        : Array.isArray(workflow.on) ? workflow.on : Object.keys(workflow.on ?? {})
      if (events.some(event => ['pull_request', 'pull_request_target'].includes(event))) {
        failures.push(`${scope}: pull requests cannot publish`)
      }
    }
  }
  return failures
}
