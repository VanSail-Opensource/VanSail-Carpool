<script setup lang="ts">
import { onMounted, shallowRef } from 'vue'
import { useRouter } from 'vue-router'
import { localizedError, useI18n } from '@/i18n'
import { useBrowserSession } from './session'
import BrowserPreferences from './BrowserPreferences.vue'
import BrowserLoading from './BrowserLoading.vue'

const { t } = useI18n()
const router = useRouter()
const session = useBrowserSession()
const error = shallowRef<string | null>(null)
onMounted(async () => {
  const token = new globalThis.URLSearchParams(globalThis.location.hash.slice(1)).get('token') ?? ''
  globalThis.history.replaceState(globalThis.history.state, '', globalThis.location.pathname)
  if (!token) { error.value = t('browser.linkInvalid'); return }
  try { await router.replace(await session.exchange(token)) }
  catch (caught) { error.value = localizedError(caught, 'browser.linkInvalid') }
})
</script>

<template>
  <BrowserLoading :status="t('browser.signingIn')" :error="error" :failure-title="t('browser.linkExpired')">
    <template #preferences><BrowserPreferences /></template>
    <UButton v-if="error" to="/" color="neutral" variant="outline" :label="t('browser.backToEntrance')" />
  </BrowserLoading>
</template>
