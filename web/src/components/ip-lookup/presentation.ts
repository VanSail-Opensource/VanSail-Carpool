import { t } from '@/i18n'

const knownErrors = new Set(['IP_LOOKUP_INVALID_IP', 'IP_LOOKUP_DISABLED', 'IP_LOOKUP_QUOTE_CHANGED', 'IP_LOOKUP_REFRESH_UNAVAILABLE', 'IP_LOOKUP_CONFIGURATION_REQUIRED', 'IP_LOOKUP_CREDENTIAL_REQUIRED', 'IP_LOOKUP_INVALID_CONFIG', 'IP_LOOKUP_ALL_PROVIDERS_FAILED', 'IP_LOOKUP_BUSY', 'IP_LOOKUP_CONFLICT', 'INSUFFICIENT_BALANCE'])

export function ipLookupErrorText(code: string): string {
  return t(`ipLookup.errors.${knownErrors.has(code) ? code : 'IP_LOOKUP_FAILED'}`)
}

export function providerName(id: string): string { return id.includes(':') && /^\d+:/.test(id) ? id.split(':')[0]! + ' · ' + t(`ipLookup.probes.${id.split(':')[1]}`) : ['proxy_lists', 'socks4', 'socks5', 'http'].includes(id) ? t(`ipLookup.sources.${id}`) : t(`ipLookup.providers.${id}`) }

const progressTaskSections = new Set(['topology', 'relationships', 'names', 'asns', 'scores', 'block', 'ptr', 'shodan', 'transfers', 'rpki', 'proxy', 'probes', 'subnet'])

// Live sections and the refresh phase use ipLookupProgress labels; anything
// else is a reputation provider id.
export function taskLabel(key: string): string {
  return progressTaskSections.has(key) || key === 'live' ? t(`ipLookupProgress.${key}`) : providerName(key)
}
export function sourceText(source: string | undefined): string {
  return source?.split(',').map(value => value.split(':').map(providerName).join(' · ')).join(', ') ?? ''
}
