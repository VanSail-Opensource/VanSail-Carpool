<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { browserAccessApi, type AdminAccessLink } from '@/api/browserAccess'
import { localizedError, useI18n } from '@/i18n'
import { openExternalLink } from '@/utils/telegram'
import { useSessionStore } from '@/stores/session'
import { useClipboard } from '@/composables/useClipboard'

const route = useRoute()
const session = useSessionStore()
const { t } = useI18n()
const link = shallowRef<AdminAccessLink | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const { copied, copy: copyLink, reset: resetCopy } = useClipboard()
let generation = 0
let disposed = false
const returnTo = computed(() => typeof route.query.returnTo === 'string' && route.query.returnTo.startsWith('/admin/') ? route.query.returnTo : '/admin/settings')
const current = (ticket: number) => !disposed && ticket === generation && session.isAdmin
const usable = (value: AdminAccessLink | null): value is AdminAccessLink => !!value && Number.isFinite(Date.parse(value.expiresAt)) && Date.parse(value.expiresAt) > Date.now()
async function generate(): Promise<AdminAccessLink | null> {
  if (disposed || busy.value || !session.isAdmin) return null
  const ticket = ++generation
  busy.value = true
  link.value = null
  error.value = null
  resetCopy()
  try {
    const next = await browserAccessApi.createLink(returnTo.value)
    if (!current(ticket)) return null
    if (!usable(next)) { error.value = t('browser.linkFailed'); return null }
    link.value = next
    return next
  } catch (caught) {
    if (current(ticket)) error.value = localizedError(caught, 'browser.linkFailed')
    return null
  } finally { if (current(ticket)) busy.value = false }
}
async function open(): Promise<void> {
  if (disposed || busy.value || !session.isAdmin) return
  const selected = usable(link.value) ? link.value : await generate()
  if (selected && selected === link.value) openExternalLink(selected.url)
}
async function copy(): Promise<void> {
  if (disposed || busy.value || !session.isAdmin) return
  const selected = usable(link.value) ? link.value : await generate()
  if (!selected || selected !== link.value) return
  const ticket = generation
  if (!await copyLink(selected.url) && current(ticket) && selected === link.value) error.value = t('browser.copyFailed')
}
watch([() => session.isAdmin ? session.user?.id : null, returnTo], () => {
  generation++; link.value = null; busy.value = false; error.value = null; resetCopy()
  if (session.isAdmin) void generate()
}, { immediate: true, flush: 'sync' })
onScopeDispose(() => { disposed = true; generation++; link.value = null; resetCopy() })
</script>

<template>
  <div class="admin-access-panel">
    <header class="page-header"><h1>{{ t('browser.adminAccess') }}</h1></header>
    <section class="admin-access-link">
      <UAlert v-if="error" color="warning" variant="soft" :title="error" />
      <UInput v-if="link" :model-value="link.url" readonly :aria-label="t('browser.copyLink')" class="w-full" />
      <div class="button-row"><UButton :loading="busy" :disabled="busy || !session.isAdmin" icon="i-ph-arrow-square-out-bold" :label="t('browser.openConsole')" @click="open" /><UButton color="neutral" variant="outline" icon="i-ph-copy-bold" :disabled="busy || !link" :label="t('browser.copyLink')" @click="copy" /></div>
      <div class="admin-access-meta"><p role="status">{{ copied ? t('browser.linkCopied') : t('browser.linkExpiry') }}</p><UButton variant="link" color="neutral" :disabled="busy || !session.isAdmin" :label="t('browser.regenerateLink')" @click="generate" /></div>
    </section>
  </div>
</template>

<style scoped>
.admin-access-panel { max-width: 720px; margin-inline: auto; padding: 20px 16px 48px; }
.admin-access-link { display: grid; gap: 16px; padding-block: 12px 28px; margin-bottom: 28px; border-bottom: 1px solid var(--line); }
.admin-access-meta { display: flex; justify-content: space-between; flex-wrap: wrap; align-items: center; gap: 8px; }
.admin-access-meta p { color: var(--text-muted); font-size: 12px; margin: 0; }
</style>
