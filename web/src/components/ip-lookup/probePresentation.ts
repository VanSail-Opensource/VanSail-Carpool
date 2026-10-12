import type { IPLookupReport } from '@/api/ipLookup'
import { t } from '@/i18n'
import type { DisplayEvidence } from './refusalPresentation'

type Port = NonNullable<IPLookupReport['ports']>[number]
type Finding = Port['findings'][number]

/** Keep open-port observations and detections; procedure data stays out of UI. */
export function visibleProbePorts(ports: Port[]) {
  return ports.filter(port => port.status !== 'closed').flatMap(port => {
    const findings = port.findings.filter(finding => ['confirmed', 'suspected', 'observed'].includes(finding.confidence))
    const byProtocol = new Map<string, Finding>()
    const rank = { observed: 1, suspected: 2, confirmed: 3 }
    for (const finding of findings) {
      const previous = byProtocol.get(finding.protocol)
      if (!previous || rank[finding.confidence as keyof typeof rank] > rank[previous.confidence as keyof typeof rank]) byProtocol.set(finding.protocol, finding)
    }
    const unique = [...byProtocol.values()]
    const open = port.tasks.some(task => task.key === 'tcp' && task.status === 'success')
    if (!unique.length && open) unique.push({ protocol: 'tcp', confidence: 'observed', evidence: '' } as Finding)
    return unique.length ? [{ port: port.port, findings: unique }] : []
  })
}

/** Port observations share the existing warning presentation with datacenters. */
export function portWarnings(ports: Port[]): DisplayEvidence[] {
  return visibleProbePorts(ports).flatMap(port => {
    const findings = port.findings.filter(finding => finding.confidence !== 'confirmed')
    return findings.length ? [{
      kind: 'port_observation', source: findings.map(finding => `${finding.protocol}:${finding.confidence}`).join(','),
      value: t('ipLookup.portLabel', { port: port.port }),
    }] : []
  })
}
