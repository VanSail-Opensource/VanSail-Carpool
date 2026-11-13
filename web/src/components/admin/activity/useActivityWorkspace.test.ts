import { defineComponent, h, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type BetGame, type LuckyDrawAdmin } from '@/api/features'
import { adminOperationsApi } from '@/api/adminOperations'
import { t } from '@/i18n'
import { draftFromDraw, serializeDraw } from './drawDraft'
import { useActivityWorkspace } from './useActivityWorkspace'

const cache = vi.hoisted(() => ({ restoreItems: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const game: BetGame = { id: 'game-one', name: 'Reviewed game', icon: 'coin', description: '', enabled: true, winChanceBps: 5000, returnMultiplierBps: 20000, minimumStakeMinor: '100', maximumStakeMinor: '500' }
const draw: LuckyDrawAdmin = { id: 'draw-one', name: 'Reviewed draw', description: '', kind: 'instant', status: 'open', enabled: true, feeTxbMinor: '100', expectedParticipation: 100, seats: 0, prizes: [{ id: 'prize-one', name: 'No prize', probabilityBps: 10000, reward: { kind: 'none' } }], createdAt: '2026-11-13T00:00:00Z', updatedAt: '2026-11-13T00:00:00Z' }
function render() {
  let state!: ReturnType<typeof useActivityWorkspace>
  const wrapper = mount(defineComponent({ setup() { state = useActivityWorkspace(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => {
  cache.restoreItems.mockReset().mockReturnValue(false)
  vi.spyOn(featuresApi, 'getAdminActivityGames').mockResolvedValue({ items: [game] })
  vi.spyOn(featuresApi, 'getAdminLuckyDraws').mockResolvedValue({ items: [draw] })
  vi.spyOn(adminOperationsApi, 'getCatalogOptions').mockResolvedValue({ combos: [], squads: [] })
})
afterEach(() => vi.restoreAllMocks())

describe('administrator activity ordering', () => {
  it('retires the cached initial inventory and blocks repeated game writes while pending', async () => {
    cache.restoreItems.mockImplementation((path: string, target: ShallowRef<unknown[]>) => { target.value = path.endsWith('activity-games') ? [game] : [draw]; return true })
    let releaseRead!: (value: { items: BetGame[] }) => void
    vi.mocked(featuresApi.getAdminActivityGames).mockReturnValueOnce(new Promise(done => { releaseRead = done }))
    let releaseSave!: (value: BetGame) => void
    const save = vi.spyOn(featuresApi, 'saveAdminActivityGame').mockReturnValue(new Promise(done => { releaseSave = done }))
    const { state, wrapper } = render()
    state.editGame(game)
    state.gameDraft.name = 'Confirmed game'
    const pending = state.saveGame()
    await state.saveGame()
    state.editGame({ ...game, id: 'replacement' })
    expect(save).toHaveBeenCalledOnce()
    expect(state.editingGameId.value).toBe(game.id)
    const saved = { ...game, name: 'Confirmed game' }
    vi.mocked(featuresApi.getAdminActivityGames).mockResolvedValue({ items: [saved] })
    releaseSave(saved)
    await pending
    releaseRead({ items: [game] })
    await flushPromises()
    expect(state.games.value).toEqual([saved])
    expect(state.editingGameId.value).toBeUndefined()
    expect(state.error.value).toBeNull()
    wrapper.unmount()
  })

  it('keeps a confirmed game when refresh fails and identifies the read failure', async () => {
    const save = vi.spyOn(featuresApi, 'saveAdminActivityGame').mockResolvedValue({ ...game, name: 'Confirmed game' })
    const { state, wrapper } = render()
    await flushPromises()
    state.editGame(game)
    vi.mocked(featuresApi.getAdminActivityGames).mockRejectedValueOnce(new Error('read unavailable'))
    await state.saveGame()
    expect(save).toHaveBeenCalledOnce()
    expect(state.games.value[0]?.name).toBe('Confirmed game')
    expect(state.editingGameId.value).toBeUndefined()
    expect(state.error.value).toBe(t('adminActivityManagement.loadFailed'))
    wrapper.unmount()
  })

  it('does not replace the draw editor or issue a second save during a pending write', async () => {
    let release!: (value: LuckyDrawAdmin) => void
    const save = vi.spyOn(featuresApi, 'saveAdminLuckyDraw').mockReturnValue(new Promise(done => { release = done }))
    const { state, wrapper } = render()
    await flushPromises()
    state.editDraw(draw)
    const draft = draftFromDraw(draw)
    draft.name = 'Confirmed draw'
    const input = serializeDraw(draft)!
    const pending = state.saveDraw(input)
    await state.saveDraw(input)
    state.editDraw({ ...draw, id: 'replacement' })
    expect(save).toHaveBeenCalledExactlyOnceWith(draw.id, input)
    expect(state.editingDraw.value?.id).toBe(draw.id)
    const saved = { ...draw, name: 'Confirmed draw' }
    vi.mocked(featuresApi.getAdminLuckyDraws).mockResolvedValue({ items: [saved] })
    release(saved)
    await pending
    expect(state.editingDraw.value).toEqual(saved)
    expect(state.draws.value).toEqual([saved])
    wrapper.unmount()
  })

  it.each(['game', 'instant', 'raffle'] as const)('retains confirmed %s removal when the following inventory read fails', async kind => {
    const targetDraw = { ...draw, kind: kind === 'raffle' ? 'raffle' as const : 'instant' as const }
    vi.mocked(featuresApi.getAdminLuckyDraws).mockResolvedValue({ items: [targetDraw] })
    const remove = kind === 'game' ? vi.spyOn(featuresApi, 'deleteAdminActivityGame') : vi.spyOn(featuresApi, 'deleteAdminLuckyDraw')
    remove.mockResolvedValue(undefined)
    const { state, wrapper } = render()
    await flushPromises()
    state.deleting.value = { kind: kind === 'game' ? 'game' : 'draw', id: kind === 'game' ? game.id : draw.id, name: 'Reviewed target' }
    vi.mocked(featuresApi.getAdminActivityGames).mockRejectedValueOnce(new Error('read unavailable'))
    await state.removeActivity()
    expect(remove).toHaveBeenCalledOnce()
    expect(state.deleting.value).toBeNull()
    expect(state.error.value).toBe(t('adminActivityManagement.loadFailed'))
    if (kind === 'game') expect(state.games.value).toEqual([])
    else if (kind === 'instant') expect(state.draws.value).toEqual([])
    else expect(state.draws.value[0]).toMatchObject({ id: draw.id, status: 'cancelled', enabled: false })
    wrapper.unmount()
  })

  it.each([false, true])('retains publication acceptance without downgrading newer state (newer=%s)', async newer => {
    const raffle: LuckyDrawAdmin = { ...draw, kind: 'raffle', status: 'draft', enabled: false }
    vi.mocked(featuresApi.getAdminLuckyDraws).mockResolvedValue({ items: [raffle] })
    let release!: () => void
    const publish = vi.spyOn(featuresApi, 'publishAdminLuckyDraw').mockReturnValue(new Promise<void>(done => { release = done }))
    const { state, wrapper } = render()
    await flushPromises()
    const pending = state.publishDraw(raffle.id)
    if (newer) state.draws.value = [{ ...raffle, status: 'open', enabled: true }]
    vi.mocked(featuresApi.getAdminActivityGames).mockRejectedValueOnce(new Error('read unavailable'))
    release()
    await pending
    expect(state.draws.value[0]?.status).toBe(newer ? 'open' : 'publishing')
    expect(state.error.value).toBe(t('adminActivityManagement.loadFailed'))
    await state.publishDraw(raffle.id)
    expect(publish).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('retires pending writes on unmount without refreshing the replacement workspace', async () => {
    let release!: (value: BetGame) => void
    vi.spyOn(featuresApi, 'saveAdminActivityGame').mockReturnValue(new Promise(done => { release = done }))
    const { state, wrapper } = render()
    await flushPromises()
    state.editGame(game)
    const pending = state.saveGame()
    wrapper.unmount()
    release({ ...game, name: 'Retired result' })
    await pending
    expect(state.games.value).toEqual([game])
    expect(featuresApi.getAdminActivityGames).toHaveBeenCalledOnce()
  })
})
