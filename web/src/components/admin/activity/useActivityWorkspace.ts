import { restoreItems } from '@/api/cache/restore'
import { computed, onMounted, onScopeDispose, reactive, shallowRef } from 'vue'
import type { AdminStatistics, BetGame, LuckyDrawAdmin, LuckyDrawWrite, StatisticsQuery } from '@/api/features'
import { featuresApi } from '@/api/features'
import { adminOperationsApi, type AdminCatalogOptions } from '@/api/adminOperations'
import { localizedError, useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

export function useActivityWorkspace() {
  const latest = createLatestRequest()
  let disposed = false
  let hydrated = false
  const games = shallowRef<BetGame[]>([])
  const draws = shallowRef<LuckyDrawAdmin[]>([])
  const catalogOptions = shallowRef<AdminCatalogOptions>({ combos: [], squads: [] })
  const editingGameId = shallowRef<string | null | undefined>(undefined)
  const editingDraw = shallowRef<LuckyDrawAdmin | null | undefined>(undefined)
  const loading = shallowRef(true)
  const busy = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const gameDraft = reactive({ name: '', icon: 'dice', description: '', winChancePercent: 50, minStake: '1.00', maxStake: '20.00', returnMultiplier: 2, enabled: true })
  const statisticsTarget = shallowRef<{ kind: 'game' | 'draw'; id: string; title: string } | null>(null)
  const deleting = shallowRef<{ kind: 'game' | 'draw'; id: string; name: string } | null>(null)
  const { t } = useI18n()
  const activeTab = shallowRef('games')
  const tabs = computed(() => [{ value: 'games', label: t('adminActivityManagement.games') }, { value: 'draws', label: t('adminActivityManagement.draws') }])
  const gameColumns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'terms', header: t('adminWorkspace.details') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
  const drawColumns = computed(() => [{ accessorKey: 'name', header: t('adminWorkspace.name') }, { id: 'type', header: t('adminWorkspace.type') }, { id: 'price', header: t('adminWorkspace.price') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])
  const iconItems = computed(() => ['dice', 'coin', 'cards', 'target', 'trophy', 'lightning', 'sparkle'].map((value) => ({ value, label: t(`adminActivityManagement.icons.${value}`) })))
  const comboItems = computed(() => catalogOptions.value.combos.map((combo) => ({ value: combo.id, label: combo.name })))
  const squadItems = computed(() => catalogOptions.value.squads.map((squad) => ({ value: squad.groupId, label: squad.name })))

  function loadStatistics(query: StatisticsQuery): Promise<AdminStatistics> {
    if (!statisticsTarget.value) return Promise.reject(new Error(t('adminActivityManagement.chooseActivity')))
    return statisticsTarget.value.kind === 'game'
      ? featuresApi.getAdminActivityGameStatistics(statisticsTarget.value.id, query)
      : featuresApi.getAdminLuckyDrawStatistics(statisticsTarget.value.id, query)
  }

  async function runMutation<T>(task: () => Promise<T>, apply: (value: T) => void, errorKey: string): Promise<boolean> {
    if (disposed || busy.value || loading.value) return false
    const ticket = latest.begin()
    busy.value = true
    error.value = null
    try {
      const value = await task()
      if (!latest.isCurrent(ticket)) return false
      apply(value)
      return true
    } catch (caught) {
      if (latest.isCurrent(ticket)) error.value = localizedError(caught, errorKey)
      return false
    } finally {
      if (latest.isCurrent(ticket)) busy.value = false
    }
  }

  async function removeActivity(): Promise<void> {
    const target = deleting.value
    if (!target || (target.kind === 'draw' && draws.value.some(draw => draw.id === target.id && draw.kind === 'raffle' && draw.status === 'cancelled'))) return
    if (await runMutation(
      () => target.kind === 'game' ? featuresApi.deleteAdminActivityGame(target.id) : featuresApi.deleteAdminLuckyDraw(target.id),
      () => {
        if (target.kind === 'game') games.value = games.value.filter(game => game.id !== target.id)
        else draws.value = draws.value.filter(draw => draw.id !== target.id || draw.kind === 'raffle')
          .map(draw => draw.id === target.id ? { ...draw, status: 'cancelled' as const, enabled: false } : draw)
        if (deleting.value === target) deleting.value = null
      },
      'adminActivityManagement.deleteFailed',
    )) await load()
  }

  async function load(): Promise<void> {
    if (disposed || busy.value) return
    const ticket = latest.begin()
    loading.value = hydrated || ![
      restoreItems('/api/v1/admin/activity-games', games),
      restoreItems('/api/v1/admin/lucky-draw', draws),
    ].every(Boolean)
    hydrated = true
    error.value = null
    try {
      const [gameResponse, drawResponse, options] = await Promise.all([
        featuresApi.getAdminActivityGames(), featuresApi.getAdminLuckyDraws(),
        adminOperationsApi.getCatalogOptions().catch(() => null),
      ])
      if (!latest.isCurrent(ticket)) return
      games.value = gameResponse.items
      draws.value = drawResponse.items
      catalogOptions.value = options ?? { combos: [], squads: [] }
      if (!options) error.value = t('adminDrawRedesign.catalogUnavailable')
    } catch (caught) {
      if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'adminActivityManagement.loadFailed')
    } finally {
      if (latest.isCurrent(ticket)) loading.value = false
    }
  }

  function editGame(game?: BetGame): void {
    if (disposed || busy.value || loading.value) return
    editingGameId.value = game?.id ?? null
    Object.assign(gameDraft, game ? {
      name: game.name, icon: game.icon, description: game.description,
      winChancePercent: game.winChanceBps / 100,
      minStake: txbInputFromMinor(game.minimumStakeMinor), maxStake: txbInputFromMinor(game.maximumStakeMinor),
      returnMultiplier: game.returnMultiplierBps / 10000, enabled: game.enabled,
    } : { name: '', icon: 'dice', description: '', winChancePercent: 50, minStake: '1.00', maxStake: '20.00', returnMultiplier: 2, enabled: true })
  }
  function editDraw(draw?: LuckyDrawAdmin): void {
    if (!disposed && !busy.value && !loading.value) editingDraw.value = draw ?? null
  }

  async function saveGame(): Promise<void> {
    if (disposed || busy.value || loading.value || editingGameId.value === undefined) return
    const minimumStakeMinor = moneyFromTxbInput(gameDraft.minStake)
    const maximumStakeMinor = moneyFromTxbInput(gameDraft.maxStake)
    if (!minimumStakeMinor || !maximumStakeMinor) { error.value = t('amount.invalidPrecision'); return }
    const id = editingGameId.value
    if (await runMutation(() => featuresApi.saveAdminActivityGame(id, {
      name: gameDraft.name, icon: gameDraft.icon, description: gameDraft.description,
      winChanceBps: Math.round(gameDraft.winChancePercent * 100),
      minimumStakeMinor, maximumStakeMinor,
      returnMultiplierBps: Math.round(gameDraft.returnMultiplier * 10000), enabled: gameDraft.enabled,
    }), saved => {
      games.value = games.value.some(game => game.id === saved.id) ? games.value.map(game => game.id === saved.id ? saved : game) : [...games.value, saved]
      if (editingGameId.value === id) editingGameId.value = undefined
    }, 'adminActivityManagement.gameSaveFailed')) await load()
  }

  async function saveDraw(value: LuckyDrawWrite): Promise<void> {
    const target = editingDraw.value
    if (target === undefined) return
    if (await runMutation(() => featuresApi.saveAdminLuckyDraw(target?.id ?? null, value), saved => {
      draws.value = draws.value.some(draw => draw.id === saved.id) ? draws.value.map(draw => draw.id === saved.id ? saved : draw) : [...draws.value, saved]
      if (editingDraw.value === target) editingDraw.value = saved
    }, 'adminActivityManagement.drawSaveFailed')) await load()
  }
  async function publishDraw(id: string): Promise<void> {
    if (!draws.value.some(draw => draw.id === id && draw.kind === 'raffle' && draw.status === 'draft')) return
    if (await runMutation(() => featuresApi.publishAdminLuckyDraw(id), () => {
      draws.value = draws.value.map(draw => draw.id === id && draw.status === 'draft'
        ? { ...draw, status: 'publishing' as const } : draw)
    }, 'adminDrawRedesign.publishError')) await load()
  }

  onMounted(() => void load())
  onScopeDispose(() => { disposed = true; latest.dispose() })
  return { games, draws, editingGameId, editingDraw, loading, busy, error, gameDraft, statisticsTarget, deleting, t, activeTab, tabs, gameColumns, drawColumns, iconItems, comboItems, squadItems, load, loadStatistics, removeActivity, editGame, editDraw, saveGame, saveDraw, publishDraw }
}
