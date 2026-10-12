import type { IPLookupReport } from '@/api/ipLookup'
import { t } from '@/i18n'
import type { DisplayEvidence } from './refusalPresentation'

type Port = NonNullable<IPLookupReport['ports']>[number]
type Finding = Port['findings'][number]

/** Keep open-port observations and detections; procedure data stays out of UI. */
export function visibleProbePorts(ports: Port[]) {
  return ports.filter(port => port.status !== 'closed').flatMap(port => {
    const findings = port.findings.filter(finding => ['confirmed', 'suspected', 'observed'].includes(finding.confidence))
    const seen = new Set<string>()
    const unique = findings.filter(finding => {
      const key = `${finding.protocol}:${finding.confidence}`
      if (seen.has(key)) return false
      seen.add(key)
      return true
    })
    const open = port.tasks.some(task => task.key === 'tcp' && task.status === 'success')
    if (!unique.length && open) unique.push({ protocol: 'tcp', confidence: 'observed', evidence: '' } as Finding)
    return unique.length ? [{ port: port.port, findings: unique }] : []
  })
}

/** Port observations share the existing warning presentation with datacenters. */
export function portWarnings(ports: Port[]): DisplayEvidence[] {
  return visibleProbePorts(ports).flatMap(port => port.findings.filter(finding => finding.confidence !== 'confirmed').map(finding => ({
    kind: 'port_observation', source: finding.protocol,
    value: t('ipLookup.portWarning', { port: port.port, confidence: t(`ipLookup.confidence.${finding.confidence}`) }),
  })))
}
