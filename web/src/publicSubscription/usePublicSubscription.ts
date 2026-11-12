import { computed, onMounted, onScopeDispose, shallowRef, watch } from 'vue'
import { ApiError } from '@/api/http'
import { publicSubscriptionMetadata, type XNodeSubscription } from '@/api/xnodeMember'
import { copyText } from '@/utils/browserClipboard'

export type SubscriptionFormat = 'base64' | 'native' | 'xray' | 'sing-box' | 'clash' | 'stash'
export function usePublicSubscription(token: string) {
  const metadata = shallowRef<XNodeSubscription | null>(null)
  const loading = shallowRef(true)
  const failed = shallowRef(false)
  const format = shallowRef<SubscriptionFormat>('base64')
  const copied = shallowRef(false)
  const copyFailed = shallowRef(false)
  const link = computed(() => {
    if (loading.value || failed.value || metadata.value?.ready === false || !metadata.value?.active || !metadata.value.formats[format.value]) return ''
    const url = new URL(metadata.value.downloadUrl || `/api/v1/subscriptions/${encodeURIComponent(token)}`, location.origin)
    url.searchParams.set('format', format.value)
    return url.href
  })
  const importLink = computed(() => {
    if (!link.value) return ''
    if (format.value === 'sing-box') return `sing-box://import-remote-profile?url=${encodeURIComponent(link.value)}#${encodeURIComponent(metadata.value?.username ?? '')}`
    if (format.value === 'clash') return `clash://install-config?url=${encodeURIComponent(link.value)}`
    return ''
  })
  let controller: AbortController | undefined
  let active = true
  let timer: ReturnType<typeof setTimeout> | undefined
  let copyGeneration = 0

  function stopPolling(): void {
    if (timer !== undefined) clearTimeout(timer)
    timer = undefined
  }
  async function load(): Promise<void> {
    if (!active) return
    stopPolling()
    controller?.abort()
    controller = new AbortController()
    const current = controller
    loading.value = true
    failed.value = false
    if (!/^[A-Za-z0-9_-]{43}$/.test(token)) {
      metadata.value = null
      loading.value = false
      failed.value = true
      return
    }
    try {
      const value = await publicSubscriptionMetadata(token, current.signal)
      if (!active || current !== controller) return
      metadata.value = value
      if (!value.formats[format.value]) format.value = (['base64', 'native', 'xray', 'sing-box', 'clash', 'stash'].find(key => value.formats[key]) as SubscriptionFormat | undefined) ?? 'base64'
    } catch (caught) {
      if (!active || current !== controller) return
      failed.value = true
      if (caught instanceof ApiError && [401, 403, 404, 410].includes(caught.status)) metadata.value = null
    } finally {
      if (active && current === controller) {
        loading.value = false
        if (!failed.value && metadata.value?.ready === false && document.visibilityState !== 'hidden') {
          timer = setTimeout(() => void load(), 2000)
        }
      }
    }
  }
  function visibilityChanged(): void {
    stopPolling()
    if (document.visibilityState !== 'hidden' && !loading.value && !failed.value && metadata.value?.ready === false) void load()
  }
  watch(link, () => {
    copyGeneration++
    copied.value = false
    copyFailed.value = false
  }, { flush: 'sync' })
  async function copy(): Promise<void> {
    const generation = ++copyGeneration
    const value = link.value
    copied.value = false
    copyFailed.value = false
    if (!value) return
    const result = await copyText(value)
    if (!active || generation !== copyGeneration || link.value !== value) return
    copied.value = result
    copyFailed.value = !result
  }
  onMounted(() => {
    document.addEventListener('visibilitychange', visibilityChanged)
    void load()
  })
  onScopeDispose(() => {
    active = false
    copyGeneration++
    controller?.abort()
    stopPolling()
    document.removeEventListener('visibilitychange', visibilityChanged)
  })
  return { metadata, loading, failed, format, copied, copyFailed, link, importLink, load, copy }
}
