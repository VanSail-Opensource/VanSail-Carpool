import { readFileSync, readdirSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import process from 'node:process'
import { parse } from 'yaml'
import { auditWorkflow } from './workflow-policy.mjs'

const directory = fileURLToPath(new URL('../../.github/workflows/', import.meta.url))
const failures = []
for (const name of readdirSync(directory).filter(name => /\.ya?ml$/.test(name))) {
  try {
    failures.push(...auditWorkflow(name, parse(readFileSync(`${directory}/${name}`, 'utf8'))))
  } catch {
    failures.push(`${name}: invalid workflow YAML`)
  }
}
if (failures.length) {
  process.stderr.write(`${failures.join('\n')}\n`)
  process.exitCode = 1
} else {
  process.stdout.write('Workflow action pins, credential handling and permissions passed.\n')
}
