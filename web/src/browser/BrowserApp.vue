<script setup lang="ts">
import { computed, onMounted, onUnmounted, provide } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useRoute } from 'vue-router'
import { useI18n } from '@/i18n'
import { adminOperationsApi } from '@/api/adminOperations'
import { operationReceiptLoaderKey } from '@/composables/operationReceiptLoader'
import BrowserConsole from './BrowserConsole.vue'
import { useBrowserSession } from './session'

const route = useRoute()
provide(operationReceiptLoaderKey, adminOperationsApi.getOperation)
const session = useBrowserSession()
const { t, locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
function refresh(): void { if (globalThis.document.visibilityState === 'visible') void session.bootstrap(true).then(() => session.renew()) }
function expired(): void { void session.bootstrap(true) }
let timer: ReturnType<typeof globalThis.setInterval> | undefined
onMounted(() => {
  globalThis.window.addEventListener('focus', refresh)
  globalThis.document.addEventListener('visibilitychange', refresh)
  globalThis.window.addEventListener('txc:browser-session-expired', expired)
  timer = globalThis.setInterval(() => { if (globalThis.document.visibilityState === 'visible') void session.renew() }, 30 * 60_000)
})
onUnmounted(() => {
  globalThis.window.removeEventListener('focus', refresh)
  globalThis.document.removeEventListener('visibilitychange', refresh)
  globalThis.window.removeEventListener('txc:browser-session-expired', expired)
  if (timer) globalThis.clearInterval(timer)
})
</script>

<template>
  <UApp :locale="uiLocale" :toaster="{ position: 'bottom-right' }">
    <div v-if="!route.matched.length" class="browser-access" role="status"><USkeleton class="h-8 w-48" /><UProgress class="w-48" :aria-label="t('browser.signingIn')" /></div>
    <BrowserConsole v-else-if="route.meta.console"><RouterView /></BrowserConsole>
    <RouterView v-else />
    <UModal :open="session.expired && route.meta.console === true" :dismissible="false" :close="false" :title="t('browser.sessionExpired')" :description="t('browser.sessionExpiredHint')" :ui="{ overlay: 'z-[100]', content: 'z-[101]' }">
      <template v-if="session.error" #body><UAlert color="warning" variant="soft" :title="session.error" /></template>
      <template #footer>
        <UButton :to="session.config?.miniAppUrl || undefined" :disabled="!session.config?.miniAppUrl" external target="_blank" rel="noopener noreferrer" :label="t('browser.linkExpired')" icon="i-ph-sign-in-bold" />
        <UButton color="neutral" variant="outline" :label="t('common.tryAgain')" @click="refresh" />
      </template>
    </UModal>
  </UApp>
</template>
