<script setup lang="ts">
import { computed, onScopeDispose, reactive, shallowRef } from 'vue'
import { useRoute } from 'vue-router'
import { pricingExportsApi, type PricingExportScope } from '@/api/pricingExports'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useI18n } from '@/i18n'
const { t } = useI18n()
const route = useRoute()
const draft = reactive({ from: String(route.query.from ?? ''), until: String(route.query.until ?? ''), users: String(route.query.users ?? ''), nodes: String(route.query.nodes ?? ''), products: String(route.query.products ?? ''), category: '', format: 'jsonl' as 'csv' | 'jsonl', includeUsers: false, includeFinancial: false, pseudonymize: true, reason: '', authorize: false })
const busy = shallowRef(false)
const error = shallowRef('')
const completed = shallowRef(false)
let controller: globalThis.AbortController | null = null
onScopeDispose(() => controller?.abort())
const sensitive = computed(() => draft.includeUsers || draft.includeFinancial || draft.users.trim() !== '')
const categories = computed(() => ['', 'raw', 'features', 'forecasts', 'pricing', 'operations', 'validation'].map(value => ({ value, label: t(`pricing.exportCategories.${value || 'all'}`) })))
const enabled = computed(() => !busy.value && (!sensitive.value || draft.authorize && draft.reason.trim() !== ''))
const ids = (value: string) => value.split(',').map(id => id.trim()).filter(Boolean)
async function download(): Promise<void> {
  if (!enabled.value) return
  busy.value = true
  error.value = ''
  completed.value = false
  controller = new globalThis.AbortController()
  try {
    // The native picker must run in the click gesture, before authorization I/O.
    const picker = (globalThis as typeof globalThis & { showSaveFilePicker?: (options: { suggestedName: string }) => Promise<globalThis.FileSystemFileHandle> }).showSaveFilePicker
    const handle = picker ? await picker({ suggestedName: 'bapo-export.zip' }) : null
    const scope: PricingExportScope = { users: ids(draft.users), nodes: ids(draft.nodes), products: ids(draft.products), category: draft.category, format: draft.format, includeUsers: draft.includeUsers, includeFinancial: draft.includeFinancial, pseudonymize: draft.pseudonymize }
    if (draft.from) scope.from = `${draft.from}T00:00:00Z`
    if (draft.until) scope.until = `${draft.until}T00:00:00Z`
    const token = sensitive.value ? (await pricingExportsApi.authorize(scope, draft.reason.trim(), controller.signal)).token : ''
    const response = await pricingExportsApi.stream(scope, token, controller.signal)
    if (handle && response.body) {
      const writable = await handle.createWritable()
      await response.body.pipeTo(writable, { signal: controller.signal })
    } else {
      const chunks: Uint8Array<ArrayBuffer>[] = []; let size = 0
      const reader = response.body?.getReader()
      if (!reader) throw new Error('stream_unavailable')
      try { for (;;) { const { done, value } = await reader.read(); if (done) break; size += value.byteLength; if (size > 32 * 1024 * 1024) { await reader.cancel(); throw new Error('download_memory_limit') }; chunks.push(value.slice() as Uint8Array<ArrayBuffer>) } } finally { reader.releaseLock() }
      const blob = new globalThis.Blob(chunks, { type: 'application/zip' })
      const url = globalThis.URL.createObjectURL(blob)
      const link = globalThis.document.createElement('a')
      link.href = url
      link.download = 'bapo-export.zip'
      link.click()
      globalThis.setTimeout(() => globalThis.URL.revokeObjectURL(url), 60000)
    }
    completed.value = true
  } catch (caught) { if ((caught as Error).name !== 'AbortError') error.value = t('pricing.exportError') }
  finally { busy.value = false; controller = null; draft.authorize = false }
}
</script>
<template>
  <section class="pricing-exports">
    <h2>{{ t('pricing.exports') }}</h2>
    <p>{{ t('pricing.exportScopeHint') }}</p>
    <form class="pricing-exports__form" @submit.prevent="download">
      <UFormField :label="t('pricing.from')"><UInput v-model="draft.from" type="date" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.until')"><UInput v-model="draft.until" type="date" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.userIds')"><UInput v-model="draft.users" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.productIds')"><UInput v-model="draft.products" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.nodeIds')"><UInput v-model="draft.nodes" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.category')"><USelect v-model="draft.category" :items="categories" class="w-full" /></UFormField>
      <UFormField :label="t('pricing.format')"><USelect v-model="draft.format" :items="[{ value: 'csv', label: 'CSV' }, { value: 'jsonl', label: 'JSONL' }]" class="w-full" /></UFormField>
      <div class="pricing-exports__checks"><UCheckbox v-model="draft.includeUsers" :label="t('pricing.includeUsers')" /><UCheckbox v-model="draft.includeFinancial" :label="t('pricing.includeFinancial')" /><UCheckbox v-model="draft.pseudonymize" :label="t('pricing.pseudonymize')" /></div>
      <template v-if="sensitive"><UFormField class="pricing-exports__wide" :label="t('pricing.reason')" required><UInput v-model="draft.reason" :maxlength="1000" class="w-full" /></UFormField><UCheckbox v-model="draft.authorize" class="pricing-exports__wide" :label="t('pricing.authorizeSensitive')" /></template>
      <div class="pricing-exports__actions"><UButton type="submit" :label="t('pricing.download')" :disabled="!enabled" :loading="busy" /><UButton v-if="busy" color="neutral" variant="outline" :label="t('common.cancel')" @click="controller?.abort()" /></div>
    </form>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice><InlineNotice v-if="completed" tone="success">{{ t('pricing.exportComplete') }}</InlineNotice>
    <p>{{ t('pricing.retentionHint') }}</p>
  </section>
</template>
<style scoped>
.pricing-exports { display: grid; gap: 1.25rem; max-width: 60rem; min-width: 0; }.pricing-exports h2, .pricing-exports p { margin: 0; }.pricing-exports h2 { font-size: 1.2rem; }.pricing-exports p { color: var(--text-muted); font-size: .875rem; }
.pricing-exports__form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }.pricing-exports__wide, .pricing-exports__checks, .pricing-exports__actions { grid-column: 1 / -1; }.pricing-exports__checks, .pricing-exports__actions { display: flex; flex-wrap: wrap; gap: 1rem; }
@media (max-width: 600px) { .pricing-exports__form { grid-template-columns: 1fr; } }
</style>
