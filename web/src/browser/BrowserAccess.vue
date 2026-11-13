<script setup lang="ts">
import { onMounted, onScopeDispose, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { ApiError } from '@/api/http'
import { localizedError, useI18n } from '@/i18n'
import { useBrowserSession } from './session'
import BrowserPreferences from './BrowserPreferences.vue'
import BrowserLoading from './BrowserLoading.vue'

const { t } = useI18n()
const router = useRouter()
const session = useBrowserSession()
const error = shallowRef<string | null>(null)
const retryable = shallowRef(false)
const invalidLink = shallowRef(false)
const busy = shallowRef(false)
const controller = new globalThis.AbortController()
let token = ''

async function exchange(): Promise<void> {
  if (busy.value || !token || controller.signal.aborted) return
  busy.value = true
  error.value = null
  retryable.value = false
  try {
    const destination = await session.exchange(token, controller.signal)
    token = ''
    if (!controller.signal.aborted) await router.replace(destination)
  } catch (caught) {
    if (controller.signal.aborted) return
    const temporary = !(caught instanceof ApiError) || caught.status >= 500 || caught.status === 408 || caught.status === 429
    retryable.value = Boolean(token) && temporary
    invalidLink.value = !temporary
    if (!retryable.value) token = ''
    error.value = localizedError(caught, invalidLink.value ? 'browser.linkInvalid' : 'browser.signInUnavailable')
  } finally {
    if (!controller.signal.aborted) busy.value = false
  }
}

onMounted(() => {
  token = new globalThis.URLSearchParams(globalThis.location.hash.slice(1)).get('token') ?? ''
  globalThis.history.replaceState(globalThis.history.state, '', globalThis.location.pathname)
  if (!token) { invalidLink.value = true; error.value = t('browser.linkInvalid'); return }
  void exchange()
})
onScopeDispose(() => { token = ''; controller.abort() })
</script>

<template>
  <BrowserLoading :status="t('browser.signingIn')" :error="error" :failure-title="t(invalidLink ? 'browser.linkExpired' : 'browser.startupFailed')">
    <template #preferences><BrowserPreferences /></template>
    <UButton v-if="error && retryable" :disabled="busy" :label="t('common.tryAgain')" @click="exchange" />
    <UButton v-if="error" to="/" color="neutral" variant="outline" :label="t('browser.backToEntrance')" />
  </BrowserLoading>
</template>
