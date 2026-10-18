import { computed, onMounted, onScopeDispose, shallowRef } from 'vue'
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
    if (!metadata.value?.active || !metadata.value.formats[format.value]) return ''
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
  async function load(): Promise<void> {
    controller?.abort(); controller = new AbortController()
    const current = controller
    loading.value = true; failed.value = false
    if (!/^[A-Za-z0-9_-]{43}$/.test(token)) { loading.value = false; failed.value = true; return }
    try {
      const value = await publicSubscriptionMetadata(token, current.signal)
      if (!active || current !== controller) return
      metadata.value = value
      if (!value.formats[format.value]) format.value = (['base64', 'native', 'xray', 'sing-box', 'clash', 'stash'].find(key => value.formats[key]) as SubscriptionFormat | undefined) ?? 'base64'
    } catch { if (active && current === controller) { failed.value = true; metadata.value = null } }
    finally { if (active && current === controller) loading.value = false }
  }
  async function copy(): Promise<void> {
    copied.value = false; copyFailed.value = false
    if (link.value) { copied.value = await copyText(link.value); copyFailed.value = !copied.value }
  }
  onMounted(() => void load())
  onScopeDispose(() => { active = false; controller?.abort() })
  return { metadata, loading, failed, format, copied, copyFailed, link, importLink, load, copy }
}
