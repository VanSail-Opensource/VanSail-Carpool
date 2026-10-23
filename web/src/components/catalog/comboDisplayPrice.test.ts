import { describe, expect, it } from 'vitest'
import type { Combo, SquadProduct } from '@/api/types'
import { comboDisplayPrice } from './comboDisplayPrice'

describe('core combo price display', () => {
  it('charges each selected paid squad once and excludes included squads', () => {
    const squad = (uuid: string, minor: string) => ({ groupId: uuid, price: { currency: 'TXB', minor, display: '' } }) as SquadProduct
    const included = squad('included', '2500')
    const paid = squad('paid', '9900')
    const free = squad('free', '0')
    const combo: Pick<Combo, 'price' | 'includedSquads'> = { price: { currency: 'TXB', minor: '10001', display: '100.01 TXB' }, includedSquads: [included] }
    expect(comboDisplayPrice(combo, [paid, included, free, paid], true).minor).toBe('19901')
    expect(comboDisplayPrice(combo, [paid], false)).toBe(combo.price)
  })
  it('keeps integer precision beyond safe floating-point values', () => {
    const combo = { price: { currency: 'TXB', minor: '9007199254740991', display: '' }, includedSquads: [] } as unknown as Combo
    const paid = { groupId: 'paid', price: { minor: '12' } } as SquadProduct
    expect(comboDisplayPrice(combo, [paid], true).minor).toBe('9007199254741003')
  })
})
