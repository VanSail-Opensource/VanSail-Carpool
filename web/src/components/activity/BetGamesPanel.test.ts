import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import type { BetGame } from '@/api/features'
import { useDisplayCurrency } from '@/composables/useDisplayCurrency'
import BetGamesPanel from './BetGamesPanel.vue'

const game: BetGame = { id: 'game-1', name: 'First game', description: '', icon: 'coin', enabled: true, winChanceBps: 5000, minimumStakeMinor: '100', maximumStakeMinor: '1000', returnMultiplierBps: 20000 }
describe('eligible activity game selection', () => {
  it('retires a disabled selection and requires a current enabled game', async () => {
    useDisplayCurrency().hydrate('TXB')
    const wrapper = mount(BetGamesPanel, { props: { games: [game], busy: false } })
    expect(wrapper.find('.bet-form').exists()).toBe(true)
    await wrapper.setProps({ games: [{ ...game, enabled: false }] })
    expect(wrapper.find('.bet-form').exists()).toBe(false)
    expect(wrapper.emitted('bet')).toBeUndefined()
    await wrapper.setProps({ games: [{ ...game, enabled: false }, { ...game, id: 'game-2', name: 'Second game' }] })
    await wrapper.get('button[data-haptic="confirm"]').trigger('click')
    expect(wrapper.emitted('bet')).toEqual([[{ gameId: 'game-2', stakeTxbMinor: '100' }]])
    wrapper.unmount()
  })
  it('does not submit another bet while an action is pending', async () => {
    useDisplayCurrency().hydrate('TXB')
    const wrapper = mount(BetGamesPanel, { props: { games: [game], busy: true } })
    const submit = wrapper.get('button[data-haptic="confirm"]')
    expect(submit.attributes('disabled')).toBeDefined()
    await submit.trigger('click')
    expect(wrapper.emitted('bet')).toBeUndefined()
    wrapper.unmount()
  })
})
