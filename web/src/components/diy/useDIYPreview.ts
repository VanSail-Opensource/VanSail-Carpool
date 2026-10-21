import { onScopeDispose, shallowRef, watch } from 'vue'
import { xnodeMemberApi, type DIYWrite, type DIYPreview } from '@/api/xnodeMember'
import { createLatestRequest } from '@/utils/latestRequest'
import { localizedError } from '@/i18n'

export function useDIYPreview(body: () => DIYWrite | null, paused: () => boolean) {
  const result = shallowRef<DIYPreview | null>(null), pending = shallowRef(false)
  const error = shallowRef<string | null>(null), latest = createLatestRequest()
  let timer: ReturnType<typeof setTimeout> | undefined
  let controller: InstanceType<typeof globalThis.AbortController> | undefined
  function clear() { latest.invalidate(); controller?.abort(); if (timer) clearTimeout(timer); pending.value = false }
  watch([() => JSON.stringify(body()), paused], () => {
    clear(); result.value = null; error.value = null
    const value = body()
    if (!value || paused()) return
    pending.value = true
    timer = setTimeout(async () => {
      const ticket = latest.begin(); controller = new globalThis.AbortController()
      try {
        const response = await xnodeMemberApi.previewDIY(JSON.parse(JSON.stringify(value)) as DIYWrite, controller.signal)
        if (latest.isCurrent(ticket)) result.value = response
      } catch (caught) { if (latest.isCurrent(ticket)) error.value = localizedError(caught, 'diy.loadFailed') }
      finally { if (latest.isCurrent(ticket)) pending.value = false }
    }, 400)
  })
  onScopeDispose(() => { clear(); latest.dispose() })
  return { result, pending, error }
}
