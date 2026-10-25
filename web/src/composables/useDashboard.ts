import { computed, inject, onMounted, onScopeDispose, provide, readonly, shallowRef, type InjectionKey } from 'vue'

import { api } from '@/api/client'
import { restoreRef } from '@/api/cache/restore'
import { localizedError } from '@/i18n'
import type { Dashboard } from '@/api/types'
import { notifyHaptic } from '@/utils/telegram'
import { createLatestRequest } from '@/utils/latestRequest'
import { createNodeUsageController, type NodeUsageController } from './dashboardNodeUsage'
import { useDurableCommand } from './useDurableCommand'
import { usePurchaseValidity } from './usePurchaseValidity'

const nodeUsageControllerKey: InjectionKey<NodeUsageController> = Symbol('dashboard-node-usage')

export function useDashboard() {
  const dashboard = shallowRef<Dashboard | null>(null)
  const loading = shallowRef(true)
  const refreshing = shallowRef(false)
  const loadError = shallowRef<string | null>(null)
  const latestLoad = createLatestRequest()
  const nodeUsageController = createNodeUsageController()
  const { nodeUsage, nodeUsageLoading, nodeUsageError, nodeUsageStart, nodeUsageEnd, loadNodeUsage, setNodeUsageStart, setNodeUsageEnd } = nodeUsageController
  const revokeCommand = useDurableCommand({
    errorKey: 'errors.subscriptionRevoke',
    onTerminal: async (receipt) => {
      if (receipt.status === 'succeeded') {
        await load({ quiet: true })
        notifyHaptic('success')
      } else notifyHaptic('error')
    },
  })
  const error = computed(() => loadError.value ?? revokeCommand.error.value)
  const validity = usePurchaseValidity(
    () => [dashboard.value?.activePurchase, dashboard.value?.queuedPurchase].filter(item => item != null),
    () => load({ quiet: true }),
  )
  const currentDashboard = computed(() => {
    const value = dashboard.value
    if (!value) return null
    return {
      ...value,
      activePurchase: validity.current(value.activePurchase) ? value.activePurchase : null,
      queuedPurchase: value.queuedPurchase && validity.status(value.queuedPurchase) === 'queued' ? value.queuedPurchase : null,
    }
  })

  const hasEntitlement = computed(() => currentDashboard.value?.activePurchase != null)
  const usageRatio = computed(() => {
    if (!dashboard.value?.statistics) return 0
    const used = Number(dashboard.value.statistics.usedTrafficBytes)
    const limit = Number(dashboard.value.statistics.trafficLimitBytes)
    if (!Number.isFinite(used) || !Number.isFinite(limit) || limit <= 0) return 0
    return Math.min(1, used / limit)
  })
  const activeSquadNames = computed(() => currentDashboard.value?.activePurchase ? currentDashboard.value.activeSquadNames ?? [] : [])

  async function load(options: { quiet?: boolean } = {}): Promise<void> {
    const token = latestLoad.begin()
    if (options.quiet) refreshing.value = true
    else {
      loading.value = !restoreRef('/api/v1/dashboard', dashboard)
    }
    loadError.value = null
    try {
      const response = await api.getDashboard()
      if (latestLoad.isCurrent(token)) dashboard.value = response
    } catch (caught) {
      if (!latestLoad.isCurrent(token)) return
      loadError.value = localizedError(caught, 'errors.dashboardUnavailable')
    } finally {
      if (latestLoad.isCurrent(token)) {
        loading.value = false
        refreshing.value = false
      }
    }
  }

  async function revokeSubscription(): Promise<boolean> {
    const accepted = await revokeCommand.execute('subscription-revoke', 'subscription-revoke', api.revokeSubscription)
    if (!accepted) notifyHaptic('error')
    return accepted
  }

  provide(nodeUsageControllerKey, nodeUsageController)

  onMounted(() => void load())
  onScopeDispose(() => {
    latestLoad.dispose()
    nodeUsageController.dispose()
  })

  return {
    dashboard: currentDashboard, loading: readonly(loading), refreshing: readonly(refreshing),
    revoking: revokeCommand.busy, revokeBlocked: revokeCommand.blocksMutations,
    revokeReceipt: revokeCommand.receipt, revokeChecking: revokeCommand.checking,
    revokeError: revokeCommand.error,
    error: readonly(error),
    nodeUsage: readonly(nodeUsage), nodeUsageLoading: readonly(nodeUsageLoading),
    nodeUsageError: readonly(nodeUsageError), nodeUsageStart: readonly(nodeUsageStart),
    nodeUsageEnd: readonly(nodeUsageEnd), hasEntitlement,
    usageRatio,
    activeSquadNames,
    load,
    revokeSubscription,
    refreshRevoke: revokeCommand.refresh,
    loadNodeUsage,
    setNodeUsageStart,
    setNodeUsageEnd,
  }
}

export function useDashboardNodeUsage(): NodeUsageController {
  const injected = inject(nodeUsageControllerKey, null)
  if (injected) return injected
  const controller = createNodeUsageController()
  onScopeDispose(controller.dispose)
  return controller
}
