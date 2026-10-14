<script setup lang="ts">
import { computed, onMounted, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { browserAccessApi, type AdminAccessLink } from '@/api/browserAccess'
import { localizedError, useI18n } from '@/i18n'
import { openExternalLink } from '@/utils/telegram'

const route = useRoute()
const { t } = useI18n()
const link = shallowRef<AdminAccessLink | null>(null)
const busy = shallowRef(false)
const error = shallowRef<string | null>(null)
const copied = shallowRef(false)
const returnTo = computed(() => typeof route.query.returnTo === 'string' && route.query.returnTo.startsWith('/admin/') ? route.query.returnTo : '/admin/settings')
async function generate(): Promise<void> {
  if (busy.value) return
  busy.value = true
  error.value = null
  copied.value = false
  try { link.value = await browserAccessApi.createLink(returnTo.value) }
  catch (caught) { error.value = localizedError(caught, 'browser.linkFailed') }
  finally { busy.value = false }
}
async function open(): Promise<void> {
  if (!link.value || Date.parse(link.value.expiresAt) <= Date.now()) await generate()
  if (link.value && !error.value) openExternalLink(link.value.url)
}
async function copy(): Promise<void> {
  if (!link.value || Date.parse(link.value.expiresAt) <= Date.now()) await generate()
  if (!link.value) return
  try { await globalThis.navigator.clipboard.writeText(link.value.url); copied.value = true }
  catch { error.value = t('browser.copyFailed') }
}
onMounted(() => void generate())
</script>

<template>
  <div class="admin-access-panel">
    <header class="page-header"><h1>{{ t('browser.adminAccess') }}</h1></header>
    <section class="admin-access-link">
      <UAlert v-if="error" color="warning" variant="soft" :title="error" />
      <UInput v-if="link" :model-value="link.url" readonly :aria-label="t('browser.copyLink')" class="w-full" />
      <div class="button-row"><UButton :loading="busy" icon="i-ph-arrow-square-out-bold" :label="t('browser.openConsole')" @click="open" /><UButton color="neutral" variant="outline" icon="i-ph-copy-bold" :disabled="busy || !link" :label="t('browser.copyLink')" @click="copy" /></div>
      <div class="admin-access-meta"><p role="status">{{ copied ? t('browser.linkCopied') : t('browser.linkExpiry') }}</p><UButton variant="link" color="neutral" :disabled="busy" :label="t('browser.regenerateLink')" @click="generate" /></div>
    </section>
  </div>
</template>

<style scoped>
.admin-access-panel { max-width: 720px; margin-inline: auto; padding: 20px 16px 48px; }
.admin-access-link { display: grid; gap: 16px; padding-block: 12px 28px; margin-bottom: 28px; border-bottom: 1px solid var(--line); }
.admin-access-meta { display: flex; justify-content: space-between; flex-wrap: wrap; align-items: center; gap: 8px; }
.admin-access-meta p { color: var(--text-muted); font-size: 12px; margin: 0; }
</style>
