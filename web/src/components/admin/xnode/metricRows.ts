import { t } from '@/i18n'
import { formatBytes } from '@/utils/format'

export function metricLabel(key: string): string {
  const field = t(`nodeAdmin.fields.${key.replace(/_seconds$/, '')}`)
  if (!field.startsWith('nodeAdmin.fields.')) return field
  const collector = t(`xnode.collectors.${key}`)
  return collector.startsWith('xnode.collectors.') ? key.replace(/_/g, ' ') : collector
}

export function metricRows(value: unknown, prefix = ''): { key: string; label: string; value: string; raw: string }[] {
  if (value !== null && typeof value === 'object') return Object.entries(value).flatMap(([key, item]) => metricRows(item, prefix ? `${prefix}.${key}` : key))
  const key = prefix.split('.').at(-1) ?? ''
  const label = metricLabel(key)
  const raw = value == null ? '' : String(value)
  let formatted = raw || t('xnode.noMetrics')
  if (typeof value === 'boolean') formatted = t(value ? 'nodeAdmin.enabled' : 'nodeAdmin.disabled')
  else if (/^\d+(\.\d+)?$/.test(raw)) {
    if (/_bps$/.test(key)) formatted = t('statistics.perSecond', { value: formatBytes(Number(raw)) })
    else if (/bytes|go_heap/.test(key) || /memory|swap|disks|disk_io|interfaces|gpu/.test(prefix) && /^(total|used|free|available|cached|buffers|sent|received|memory_total|memory_used|readBytes|writeBytes)$/.test(key)) formatted = formatBytes(raw.split('.')[0])
    else if (/percent|utilization|cpu_usage/i.test(prefix)) formatted = `${Number(raw).toFixed(2)}%`
    else if (/millis/.test(key)) formatted = `${raw} ms`
    else if (/_mbps$/.test(key)) formatted = `${raw} ${t('xnode.mbps')}`
    else if (/temperature/.test(key)) formatted = `${raw} °C`
    else if (/seconds|uptime/.test(key)) formatted = `${raw} s`
    else if (typeof value === 'number' && !Number.isInteger(value)) formatted = value.toFixed(2)
  }
  const parent = prefix.includes('.') ? prefix.slice(0, prefix.lastIndexOf('.')).split('.').map(part => { const label = t(`nodeAdmin.fields.${part}`); if (!label.startsWith('nodeAdmin.fields.')) return label; const collector = t(`xnode.collectors.${part}`); return collector.startsWith('xnode.collectors.') ? part.replace(/_/g, ' ') : collector }).join(' / ') : ''
  return [{ key: prefix, label: parent ? `${parent} · ${label}` : label, value: formatted, raw }]
}
