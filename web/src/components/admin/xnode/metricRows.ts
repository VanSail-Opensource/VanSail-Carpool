import { t } from '@/i18n'
import { formatBytes } from '@/utils/format'

export function metricRows(value: unknown, prefix = ''): { key: string; label: string; value: string; raw: string }[] {
  if (value !== null && typeof value === 'object') return Object.entries(value).flatMap(([key, item]) => metricRows(item, prefix ? `${prefix}.${key}` : key))
  const key = prefix.split('.').at(-1) ?? ''
  const translated = t(`nodeAdmin.fields.${key}`)
  const label = translated.startsWith('nodeAdmin.fields.') ? key.replace(/_/g, ' ') : translated
  const raw = value == null ? '' : String(value)
  let formatted = raw || t('xnode.noMetrics')
  if (typeof value === 'boolean') formatted = t(value ? 'nodeAdmin.enabled' : 'nodeAdmin.disabled')
  else if (/^\d+(\.\d+)?$/.test(raw)) {
    if (/_bps$/.test(key)) formatted = t('statistics.perSecond', { value: formatBytes(Number(raw)) })
    else if (/bytes|go_heap/.test(key) || /memory|swap|disks|disk_io|interfaces|gpu/.test(prefix) && /^(total|used|free|available|cached|buffers|sent|received|memory_total|memory_used|readBytes|writeBytes)$/.test(key)) formatted = formatBytes(raw.split('.')[0])
    else if (/percent|utilization|cpu_usage/.test(prefix)) formatted = `${Number(raw).toFixed(2)}%`
    else if (/millis/.test(key)) formatted = `${raw} ms`
    else if (/_mbps$/.test(key)) formatted = `${raw} ${t('xnode.mbps')}`
    else if (/temperature/.test(key)) formatted = `${raw} °C`
    else if (/seconds|uptime/.test(key)) formatted = `${raw} s`
    else if (typeof value === 'number' && !Number.isInteger(value)) formatted = value.toFixed(2)
  }
  const parent = prefix.includes('.') ? prefix.slice(0, prefix.lastIndexOf('.')) : ''
  return [{ key: prefix, label: parent ? `${parent} · ${label}` : label, value: formatted, raw }]
}
