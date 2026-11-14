import { appendFileSync, readFileSync } from 'node:fs'
import { execFileSync } from 'node:child_process'
import { allBrowserScopes } from './browser-recovery-scopes.mjs'
import { selectBrowserRecovery } from './browser-recovery-selection.mjs'

const env = globalThis.process.env
let plan = { scopes: [], reasons: [] }
let number
if (env.GITHUB_EVENT_NAME === 'pull_request') {
  const pullRequest = JSON.parse(readFileSync(env.GITHUB_EVENT_PATH, 'utf8')).pull_request
  number = pullRequest?.number
  const base = pullRequest?.base?.sha
  const head = pullRequest?.head?.sha
  if (!Number.isSafeInteger(number) || number < 1 || ![base, head].every(value => /^[a-f0-9]{40}$/.test(value ?? ''))) {
    throw new Error('A valid pull request number and exact base/head commits are required')
  }
  try {
    // Array arguments and NUL-separated names keep PR filenames out of the shell.
    // --no-renames retains both sides of moves, including removal from an old area.
    const changed = execFileSync('git', ['diff', '--name-only', '--no-renames', '-z', `${base}...${head}`, '--'], { encoding: 'utf8', maxBuffer: 16 * 1024 * 1024 })
    plan = selectBrowserRecovery(changed.split('\0').filter(Boolean))
  } catch (error) {
    globalThis.console.warn(`PR change scope unavailable (${error.message}); all browser scopes are required.`)
    plan = { scopes: allBrowserScopes, reasons: [{ fallback: 'Diff unavailable; no browser coverage is skipped.' }] }
  }
  if (env.GITHUB_STEP_SUMMARY) {
    appendFileSync(env.GITHUB_STEP_SUMMARY, [
      `## Browser recovery for PR #${number}`, '',
      `Compared \`${base}\` → \`${head}\`.`, '',
      plan.scopes.length ? `Selected scopes: ${plan.scopes.map(value => `\`${value}\``).join(', ')}.` : 'No browser runtime changes; browser recovery is skipped.', '',
      'The artifact records this PR scope. Shared runtime changes expand coverage; focused feature changes select their recovery suites.', '',
    ].join('\n'))
  }
}
appendFileSync(env.GITHUB_OUTPUT, `required=${plan.scopes.length > 0}\nscopes=${JSON.stringify(plan.scopes)}\n`)
globalThis.console.log(JSON.stringify({ pullRequest: number, ...plan }, null, 2))
