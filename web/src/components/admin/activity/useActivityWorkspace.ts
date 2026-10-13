import { restoreItems } from '@/api/cache/restore'
import { computed, onMounted, reactive, shallowRef } from 'vue'
import type { AdminStatistics, BetGame, LuckyDrawAdmin, LuckyDrawWrite, StatisticsQuery } from '@/api/features'
import { featuresApi } from '@/api/features'
import { adminOperationsApi, type AdminCatalogOptions } from '@/api/adminOperations'
import { localizedError, useI18n } from '@/i18n'
import { moneyFromTxbInput, txbInputFromMinor } from '@/utils/format'

export function useActivityWorkspace() {
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
  const squadItems = computed(() => catalogOptions.value.squads.map((squad) => ({ value: squad.remnaSquadUuid, label: squad.name })))

  function loadStatistics(query: StatisticsQuery): Promise<AdminStatistics> {
    if (!statisticsTarget.value) return Promise.reject(new Error(t('adminActivityManagement.chooseActivity')))
    return statisticsTarget.value.kind === 'game'
      ? featuresApi.getAdminActivityGameStatistics(statisticsTarget.value.id, query)
      : featuresApi.getAdminLuckyDrawStatistics(statisticsTarget.value.id, query)
  }

  async function removeActivity(): Promise<void> {
    if (!deleting.value || busy.value) return
    busy.value = true
    error.value = null
    try {
      if (deleting.value.kind === 'game') await featuresApi.deleteAdminActivityGame(deleting.value.id)
      else await featuresApi.deleteAdminLuckyDraw(deleting.value.id)
      deleting.value = null
      await load()
    } catch (caught) {
      error.value = localizedError(caught, 'adminActivityManagement.deleteFailed')
    } finally {
      busy.value = false
    }
  }

  async function load(): Promise<void> {
    loading.value = ![
      restoreItems('/api/v1/admin/activity-games', games),
      restoreItems('/api/v1/admin/lucky-draw', draws),
    ].every(Boolean)
    error.value = null
    try {
      const optionsPromise = adminOperationsApi.getCatalogOptions().catch(() => null)
      const [gameResponse, drawResponse] = await Promise.all([
        featuresApi.getAdminActivityGames(),
        featuresApi.getAdminLuckyDraws(),
      ])
      games.value = gameResponse.items
      draws.value = drawResponse.items
      const options = await optionsPromise
      catalogOptions.value = options ?? { combos: [], squads: [] }
      if (!options) error.value = t('adminDrawRedesign.catalogUnavailable')
    } catch (caught) {
      error.value = localizedError(caught, 'adminActivityManagement.loadFailed')
    } finally {
      loading.value = false
    }
  }

  function editGame(game?: BetGame): void {
    editingGameId.value = game?.id ?? null
    Object.assign(gameDraft, game ? {
      name: game.name,
      icon: game.icon,
      description: game.description,
      winChancePercent: game.winChanceBps / 100,
      minStake: txbInputFromMinor(game.minimumStakeMinor),
      maxStake: txbInputFromMinor(game.maximumStakeMinor),
      returnMultiplier: game.returnMultiplierBps / 10000,
      enabled: game.enabled,
    } : { name: '', icon: 'dice', description: '', winChancePercent: 50, minStake: '1.00', maxStake: '20.00', returnMultiplier: 2, enabled: true })
  }

  async function saveGame(): Promise<void> {
    const minimumStakeMinor = moneyFromTxbInput(gameDraft.minStake)
    const maximumStakeMinor = moneyFromTxbInput(gameDraft.maxStake)
    if (!minimumStakeMinor || !maximumStakeMinor) return
    busy.value = true
    error.value = null
    try {
      await featuresApi.saveAdminActivityGame(editingGameId.value ?? null, {
        name: gameDraft.name,
        icon: gameDraft.icon,
        description: gameDraft.description,
        winChanceBps: Math.round(gameDraft.winChancePercent * 100),
        minimumStakeMinor,
        maximumStakeMinor,
        returnMultiplierBps: Math.round(gameDraft.returnMultiplier * 10000),
        enabled: gameDraft.enabled,
      })
      editingGameId.value = undefined
      games.value = (await featuresApi.getAdminActivityGames()).items
    } catch (caught) {
      error.value = localizedError(caught, 'adminActivityManagement.gameSaveFailed')
    } finally {
      busy.value = false
    }
  }

  async function saveDraw(value: LuckyDrawWrite): Promise<void> {
    busy.value = true
    error.value = null
    try {
      const saved = await featuresApi.saveAdminLuckyDraw(editingDraw.value?.id ?? null, value)
      draws.value = (await featuresApi.getAdminLuckyDraws()).items
      editingDraw.value = draws.value.find((draw) => draw.id === saved.id) ?? saved
    } catch (caught) {
      error.value = localizedError(caught, 'adminActivityManagement.drawSaveFailed')
    } finally {
      busy.value = false
    }
  }
  async function publishDraw(id: string): Promise<void> {
    if (busy.value) return
    busy.value = true
    error.value = null
    try {
      await featuresApi.publishAdminLuckyDraw(id)
      draws.value = (await featuresApi.getAdminLuckyDraws()).items
    } catch (caught) {
      error.value = localizedError(caught, 'adminDrawRedesign.publishError')
    } finally {
      busy.value = false
    }
  }

  onMounted(() => void load())
  return { games, draws, editingGameId, editingDraw, loading, busy, error, gameDraft, statisticsTarget, deleting, t, activeTab, tabs, gameColumns, drawColumns, iconItems, comboItems, squadItems, loadStatistics, removeActivity, editGame, saveGame, saveDraw, publishDraw }
}
