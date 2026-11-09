import { trafficBytesFromInput } from './format'

interface Quantity {
  value: bigint
  scale: bigint
}

const bitRateUnits: Record<string, bigint> = {
  bps: 1n,
  kbps: 1000n,
  mbps: 1000n ** 2n,
  gbps: 1000n ** 3n,
  tbps: 1000n ** 4n,
}

const byteRateUnits: Record<string, Quantity> = {
  'b/s': { value: 1n, scale: 1n },
  'kb/s': { value: 1000n, scale: 1n },
  'kib/s': { value: 1024n, scale: 1n },
  'mb/s': { value: 1000n ** 2n, scale: 1n },
  'mib/s': { value: 1024n ** 2n, scale: 1n },
  'gb/s': { value: 1000n ** 3n, scale: 1n },
  'gib/s': { value: 1024n ** 3n, scale: 1n },
  'tb/s': { value: 1000n ** 4n, scale: 1n },
  'tib/s': { value: 1024n ** 4n, scale: 1n },
}

function decimalQuantity(input: string): Quantity | null {
  const match = /^(\d+)(?:\.(\d+))?$/.exec(input.trim())
  if (!match) return null
  const fraction = match[2] ?? ''
  return { value: BigInt(match[1] + fraction), scale: 10n ** BigInt(fraction.length) }
}

function floorDivide(value: bigint, numerator: bigint, denominator: bigint): bigint {
  return value * numerator / denominator
}

// Sizes accept the same human units as the catalog traffic limit, so
// administrators can enter "1GB" or "500MiB" instead of raw bytes.
export function sizeInputFromBytes(value: string | bigint): string {
  let bytes: bigint
  try {
    bytes = typeof value === 'bigint' ? value : BigInt(value.trim())
  } catch {
    return '0 B'
  }
  if (bytes <= 0n) return '0 B'
  const units: readonly [string, bigint][] = [['TiB', 1024n ** 4n], ['GiB', 1024n ** 3n], ['MiB', 1024n ** 2n]]
  for (const [label, divisor] of units) if (bytes % divisor === 0n) return `${bytes / divisor} ${label}`
  if (bytes < 1024n ** 2n && bytes % 1024n === 0n) return `${bytes / 1024n} KiB`
  return `${bytes} B`
}

export function bytesFromSizeInput(value: string): string {
  return trafficBytesFromInput(value)
}

// Network speeds accept bit units ("100Mbps"), byte units ("12.5MB/s") and
// exact bytes per second. Bit units are decimal; KiB/MiB/GiB are binary.
export function bytesPerSecondFromInput(value: string): string {
  const match = /^(\d+(?:\.\d+)?)\s*([a-z/]*)$/i.exec(value.trim())
  if (!match) return ''
  const quantity = decimalQuantity(match[1])
  if (!quantity) return ''
  const unit = (match[2] ?? '').toLowerCase()
  if (!unit) return (quantity.value / quantity.scale).toString()
  const bits = bitRateUnits[unit]
  if (bits !== undefined) return floorDivide(quantity.value, bits, quantity.scale * 8n).toString()
  const bytes = byteRateUnits[unit]
  if (bytes) return floorDivide(quantity.value, bytes.value, quantity.scale * bytes.scale).toString()
  return ''
}

export function rateInputFromBytesPerSecond(value: string | bigint): string {
  let rate: bigint
  try {
    rate = typeof value === 'bigint' ? value : BigInt(value.trim())
  } catch {
    return '0 B/s'
  }
  if (rate <= 0n) return '0 B/s'
  if (rate % 125_000n === 0n) {
    const mbps = rate / 125_000n
    return mbps % 1000n === 0n && mbps > 0n ? `${mbps / 1000n} Gbps` : `${mbps} Mbps`
  }
  const units: readonly [string, bigint][] = [['GiB/s', 1024n ** 3n], ['MiB/s', 1024n ** 2n], ['KiB/s', 1024n]]
  for (const [label, divisor] of units) if (rate % divisor === 0n) return `${rate / divisor} ${label}`
  return `${rate} B/s`
}
