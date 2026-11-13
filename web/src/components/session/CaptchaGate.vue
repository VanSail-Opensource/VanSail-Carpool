<script setup lang="ts">
import { onMounted, onScopeDispose, shallowRef, useTemplateRef, watch } from 'vue'

import { api } from '@/api/client'
import type { Session } from '@/api/types'
import InlineNotice from '@/components/common/InlineNotice.vue'
import LanguageControl from '@/components/layout/LanguageControl.vue'
import { localizedError, useI18n } from '@/i18n'
import { loadTurnstile, type Turnstile } from '@/utils/turnstile'

const props = defineProps<{ challenge: NonNullable<Session['captcha']> }>()
const emit = defineEmits<{ verified: [session: Session] }>()
const { locale, t } = useI18n()
const container = useTemplateRef<globalThis.HTMLElement>('widget')
const loading = shallowRef(true)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
let provider: Turnstile | null = null
let widgetId: string | null = null
let disposed = false
let generation = 0

const current = (version: number) => !disposed && version === generation

async function verify(token: string, version: number): Promise<void> {
  if (busy.value || !current(version)) return
  busy.value = true
  error.value = null
  try {
    const session = await api.verifyCaptcha(token)
    if (session.captcha?.required) throw new Error(t('captcha.failed'))
    if (current(version)) emit('verified', session)
  } catch (caught) {
    if (current(version)) {
      error.value = localizedError(caught, 'captcha.failed')
      if (widgetId !== null) provider?.reset(widgetId)
    }
  } finally { if (current(version)) busy.value = false }
}

async function render(): Promise<void> {
  const version = ++generation
  busy.value = false
  loading.value = true
  error.value = null
  if (widgetId !== null) provider?.remove(widgetId)
  widgetId = null
  try {
    if (!props.challenge.siteKey) throw new Error(t('captcha.unavailable'))
    provider = await loadTurnstile()
    if (!current(version) || !container.value) return
    widgetId = provider.render(container.value, {
      sitekey: props.challenge.siteKey, action: props.challenge.action, theme: 'dark', size: 'flexible',
      language: locale.value === 'zh-CN' ? 'zh-cn' : 'en',
      callback: token => void verify(token, version),
      'error-callback': () => { if (current(version) && !busy.value) error.value = t('captcha.unavailable') },
      'expired-callback': () => { if (current(version) && !busy.value) { error.value = t('captcha.expired'); if (widgetId !== null) provider?.reset(widgetId) } },
    })
  } catch { if (current(version)) error.value = t('captcha.unavailable') }
  finally { if (current(version)) loading.value = false }
}

onMounted(() => void render())
watch([locale, () => props.challenge.siteKey, () => props.challenge.action], (next, previous) => {
  if (!busy.value || next[1] !== previous[1] || next[2] !== previous[2]) void render()
})
onScopeDispose(() => { disposed = true; generation++; if (widgetId !== null) provider?.remove(widgetId) })
</script>

<template>
  <main class="auth-screen captcha-screen" :aria-busy="loading || busy">
    <div class="auth-screen__copy">
      <p class="eyebrow">{{ t('captcha.eyebrow') }}</p>
      <h1>{{ t('captcha.title') }}</h1>
      <p>{{ t('captcha.description') }}</p>
    </div>
    <p v-if="loading || busy" role="status">{{ t(busy ? 'captcha.verifying' : 'captcha.loading') }}</p>
    <div ref="widget" class="captcha-screen__widget" />
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UButton v-if="error" :disabled="busy" icon="i-ph-arrow-clockwise" :label="t('captcha.retry')" @click="render" />
    <footer class="auth-screen__locale"><LanguageControl /></footer>
  </main>
</template>

<style scoped>
.captcha-screen { max-width: 28rem; margin-inline: auto; }
.captcha-screen__widget { width: 100%; min-width: 0; min-height: 65px; }
.captcha-screen > p { color: var(--text-muted); font-size: 0.85rem; }
</style>
