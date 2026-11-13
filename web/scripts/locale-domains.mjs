import { readdirSync } from 'node:fs'
import { resolve } from 'node:path'

export function inspectLocaleDomains(directory, manifest) {
  const failures = []
  const declared = new Set(manifest.domains)
  if (declared.size !== manifest.domains.length) failures.push('locale manifest contains duplicate domains')
  for (const locale of manifest.locales) {
    const actual = readdirSync(resolve(directory, locale))
      .filter(file => file.endsWith('.json')).map(file => file.slice(0, -5))
    for (const domain of actual) {
      if (!declared.has(domain)) failures.push(`locale ${locale}/${domain}.json is missing from the manifest`)
    }
  }
  return failures
}
