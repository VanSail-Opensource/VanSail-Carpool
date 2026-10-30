<script setup lang="ts">
import { computed, onMounted, onUnmounted, provide, shallowRef, watch } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '@/i18n'
import { adminOperationsApi } from '@/api/adminOperations'
import { operationReceiptLoaderKey } from '@/composables/operationReceiptLoader'
import BrowserConsole from './BrowserConsole.vue'
import { useBrowserSession } from './session'
import BrowserLoading from './BrowserLoading.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const route = useRoute()
const router = useRouter()
const startupTarget = globalThis.location.pathname + globalThis.location.search + globalThis.location.hash
const { reducedMotion } = useMotionPreferences()
provide(operationReceiptLoaderKey, adminOperationsApi.getOperation)
const session = useBrowserSession()
const { t, locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
const accessLeaving = shallowRef(false)
watch(() => route.name, (_next, previous) => { accessLeaving.value = previous === 'browser-access' })
async function retryStartup(): Promise<void> {
  if (session.loading) return
  await session.bootstrap(true)
  if (!session.error) await router.replace(startupTarget)
}
function refresh(): void {
  if (globalThis.document.visibilityState !== 'visible') return
  if (!route.matched.length) { void retryStartup(); return }
  void session.bootstrap(true).then(() => session.renew())
}
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
    <Transition name="browser-startup" :css="!reducedMotion" @before-leave="element => element.setAttribute('inert', '')">
      <BrowserLoading v-if="!route.matched.length" :error="session.error">
        <UButton :label="t('common.tryAgain')" :disabled="session.loading" @click="retryStartup" />
      </BrowserLoading>
    </Transition>
    <RouterView v-slot="{ Component }">
      <BrowserConsole v-if="route.meta.console"><component :is="Component" /></BrowserConsole>
      <Transition name="browser-startup" :css="!reducedMotion && accessLeaving" @before-leave="element => element.setAttribute('inert', '')">
        <component :is="Component" v-if="Component && !route.meta.console" />
      </Transition>
    </RouterView>
    <UModal :open="session.expired && route.meta.console === true" :dismissible="false" :close="false" :title="t('browser.sessionExpired')" :description="t('browser.sessionExpiredHint')" :ui="{ overlay: 'z-[100]', content: 'z-[101]' }">
      <template v-if="session.error" #body><UAlert color="warning" variant="soft" :title="session.error" /></template>
      <template #footer>
        <UButton :to="session.config?.miniAppUrl || undefined" :disabled="!session.config?.miniAppUrl" external target="_blank" rel="noopener noreferrer" :label="t('browser.linkExpired')" icon="i-ph-sign-in-bold" />
        <UButton color="neutral" variant="outline" :label="t('common.tryAgain')" @click="refresh" />
      </template>
    </UModal>
  </UApp>
</template>
