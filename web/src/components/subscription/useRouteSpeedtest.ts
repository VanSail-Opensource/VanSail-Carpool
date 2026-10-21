import { onScopeDispose, shallowRef } from 'vue'
import { xnodeMemberApi, type RouteSpeedtestResult, type RouteSpeedtestWrite } from '@/api/xnodeMember'
import { useDurableCommand } from '@/composables/useDurableCommand'

export function useRouteSpeedtest() {
  const result = shallowRef<RouteSpeedtestResult | null>(null)
  let disposed = false
  let generation = 0
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  const command = useDurableCommand({
    errorKey: 'routeSpeedtest.failed',
    loadReceipt: async id => {
      const ticket = generation
      controller?.abort(); controller = new globalThis.AbortController()
      const value = await xnodeMemberApi.routeSpeedtestResult(id, controller.signal)
      if (!disposed && ticket === generation) result.value = value
      return value.receipt
    },
  })
  async function start(request: RouteSpeedtestWrite) {
    if (command.blocksMutations.value || command.submitting.value) return
    generation++; controller?.abort()
    const body = JSON.parse(JSON.stringify(request)) as RouteSpeedtestWrite
    result.value = null
    if (await command.execute('route-speedtest', JSON.stringify(body), key => xnodeMemberApi.startRouteSpeedtest(body, key))) await command.refresh()
  }
  onScopeDispose(() => { disposed = true; controller?.abort() })
  return { command, result, start }
}
