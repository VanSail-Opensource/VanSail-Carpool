import { describe, expect, it } from 'vitest'
import { bytesFromSizeInput, bytesPerSecondFromInput, rateInputFromBytesPerSecond, sizeInputFromBytes } from './transferUnits'

describe('human transfer units', () => {
  it('converts size input to exact bytes', () => {
    expect(bytesFromSizeInput('1GB')).toBe('1073741824')
    expect(bytesFromSizeInput('500MiB')).toBe('524288000')
    expect(bytesFromSizeInput('1.5 MiB')).toBe('1572864')
    expect(bytesFromSizeInput('1024 B')).toBe('1024')
    expect(bytesFromSizeInput('nope')).toBe('')
  })
  it('converts bit and byte rates to bytes per second', () => {
    expect(bytesPerSecondFromInput('100Mbps')).toBe('12500000')
    expect(bytesPerSecondFromInput('1Gbps')).toBe('125000000')
    expect(bytesPerSecondFromInput('12.5MB/s')).toBe('12500000')
    expect(bytesPerSecondFromInput('1MiB/s')).toBe('1048576')
    expect(bytesPerSecondFromInput('250000')).toBe('250000')
    expect(bytesPerSecondFromInput('1GB')).toBe('')
  })
  it('displays editable values that round-trip without changing the policy', () => {
    expect(sizeInputFromBytes('10737418240')).toBe('10 GiB')
    expect(sizeInputFromBytes('10000000000')).toBe('10000000000 B')
    expect(rateInputFromBytesPerSecond('12500000')).toBe('100 Mbps')
    expect(rateInputFromBytesPerSecond('125000000')).toBe('1 Gbps')
    expect(rateInputFromBytesPerSecond('1048576')).toBe('1 MiB/s')
    expect(rateInputFromBytesPerSecond('1234567')).toBe('1234567 B/s')
    expect(bytesFromSizeInput(sizeInputFromBytes('10000000000'))).toBe('10000000000')
    expect(bytesPerSecondFromInput(rateInputFromBytesPerSecond('1234567'))).toBe('1234567')
  })
})
