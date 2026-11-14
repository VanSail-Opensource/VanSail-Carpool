<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import type { SubscriptionHost } from '@/api/subscription'
import CountryFlag from '@/components/common/CountryFlag.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useClipboard } from '@/composables/useClipboard'
import { subscriptionApi } from '@/api/subscription'
import { localizedError } from '@/i18n'
import { useI18n } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

const props = defineProps<{ host: Readonly<SubscriptionHost> }>()
const emit = defineEmits<{ saved: [] }>()
const { t } = useI18n()
const editOpen = shallowRef(false)
const draft = shallowRef('')
const saving = shallowRef(false)
const saveError = shallowRef<string | null>(null)
const names = createLatestRequest()
const copies = createLatestRequest()
const preview = computed(() => (draft.value.trim() || props.host.publishedName || props.host.name).replaceAll('{price}', props.host.priceLabel || ''))
function editName(): void {
  if (saving.value) return
  draft.value = props.host.nameOverride || ''
  saveError.value = null
  editOpen.value = true
}
async function saveName(): Promise<void> {
  if (saving.value) return
  const hostID = props.host.uuid
  const name = draft.value
  const generation = names.begin()
  saving.value = true
  saveError.value = null
  try {
    await subscriptionApi.saveHostName(hostID, name)
    if (!names.isCurrent(generation) || props.host.uuid !== hostID) return
    emit('saved')
    editOpen.value = false
  } catch (error) { if (names.isCurrent(generation)) saveError.value = localizedError(error, 'hostNames.saveFailed') }
  finally { if (names.isCurrent(generation)) saving.value = false }
}
const { copy, copied, reset: resetClipboard } = useClipboard()
const open = shallowRef(false)
const qr = shallowRef<string | null>(null)
const qrError = shallowRef(false)
const copyError = shallowRef(false)
const latest = createLatestRequest()
const flags = computed(() => props.host.countryCodes.length ? props.host.countryCodes : [''])
const linkError = computed(() => t('subscription.noLink'))
watch(() => [open.value, props.host.link] as const, async ([show, link]) => {
  const token = latest.begin()
  qr.value = null
  qrError.value = false
  if (!show || !link) { open.value = false; return }
  try {
    const { default: QRCode } = await import('qrcode')
    const value = await QRCode.toDataURL(link, { width: 280, margin: 3, errorCorrectionLevel: 'M', color: { dark: '#111111', light: '#ffffff' } })
    if (latest.isCurrent(token)) qr.value = value
  } catch { if (latest.isCurrent(token)) qrError.value = true }
})
watch(() => props.host.uuid, () => {
  names.invalidate()
  saving.value = false
  saveError.value = null
  editOpen.value = false
  draft.value = ''
  open.value = false
}, { flush: 'sync' })
watch(() => [props.host.uuid, props.host.link] as const, () => {
  copies.invalidate()
  resetClipboard()
  copyError.value = false
}, { flush: 'sync' })
onScopeDispose(() => { latest.dispose(); names.dispose(); copies.dispose() })
async function copyLink(): Promise<void> {
  const generation = copies.begin()
  const link = props.host.link
  copyError.value = false
  if (!link) return
  const success = await copy(link)
  if (copies.isCurrent(generation)) copyError.value = !success
}
</script>

<template>
  <li class="host-link" :class="{ 'host-link--custom': host.customRoute || host.splitXhttp }">
    <div class="host-link__row">
      <span class="host-link__flags"><CountryFlag v-for="country in flags" :key="country" :code="country" /></span>
      <span v-if="host.customRoute || host.splitXhttp" class="host-link__badges">
        <UTooltip v-if="host.customRoute" :text="t('hostNames.customRoute')">
          <span class="host-link__badge" tabindex="0" role="img" :aria-label="t('hostNames.customRoute')"><UIcon name="i-ph-path" aria-hidden="true" /></span>
        </UTooltip>
        <UTooltip v-if="host.splitXhttp" :text="t('hostNames.splitXhttp')">
          <span class="host-link__badge" tabindex="0" role="img" :aria-label="t('hostNames.splitXhttp')"><UIcon name="i-ph-arrows-split" aria-hidden="true" /></span>
        </UTooltip>
      </span>
      <UButton class="host-link__name" :disabled="saving" color="neutral" variant="link" :aria-label="t('hostNames.edit', { name: host.name })" @click="editName">{{ host.name || host.uuid }}</UButton>
      <UButton color="neutral" variant="ghost" :icon="copied ? 'i-ph-check-bold' : 'i-ph-copy'" :disabled="!host.link" :aria-label="t(copied ? 'common.copied' : 'subscription.copyHost', { name: host.name })" data-haptic="copy" @click="copyLink" />
      <UButton color="neutral" variant="ghost" icon="i-ph-qr-code" :disabled="!host.link" :aria-label="t('subscription.qrHost', { name: host.name })" data-haptic="open" @click="open = true" />
    </div>
    <UModal :open="editOpen" :close="!saving" @update:open="!saving && (editOpen = $event)" :title="t('hostNames.title')" :description="t('hostNames.hint', { placeholder: '{price}' })" :dismissible="!saving">
      <template #body>
        <form class="host-link__editor" @submit.prevent="saveName">
          <UFormField :label="t('hostNames.name')"><UInput v-model="draft" :disabled="saving" :maxlength="100" :placeholder="host.publishedName || host.name" class="w-full" /></UFormField>
          <p>{{ t('hostNames.preview', { name: preview }) }}</p>
          <InlineNotice v-if="saveError" tone="warning">{{ saveError }}</InlineNotice>
          <UButton type="submit" :disabled="saving" :loading="saving" :label="t('common.save')" />
        </form>
      </template>
    </UModal>
    <small v-if="!host.link">{{ linkError }}</small>
    <InlineNotice v-if="copyError" tone="warning">{{ t('subscription.copyFailed') }}</InlineNotice>
    <UModal v-model:open="open" :title="host.name || host.uuid" :description="t('subscription.qrHint')" close-icon="i-ph-x-bold">
      <template #body>
        <div class="host-link__qr">
          <img v-if="qr" :src="qr" :alt="t('subscription.qrHost', { name: host.name })" width="280" height="280" />
          <InlineNotice v-else-if="qrError" tone="warning">{{ t('subscription.qrFailed') }}</InlineNotice>
          <USkeleton v-else class="host-link__qr-loading" />
        </div>
      </template>
    </UModal>
  </li>
</template>

<style scoped>
.host-link { display: grid; gap: 0.4rem; padding: 0.65rem 0; border-bottom: 1px solid var(--line); }
.host-link--custom { border-inline-start: 2px solid var(--accent); padding-inline-start: .5rem; }
.host-link__editor { display: grid; gap: 1rem; }
.host-link__badges { display: inline-flex; align-items: center; gap: .2rem; flex: 0 0 auto; }
.host-link__badge { display: inline-flex; align-items: center; justify-content: center; width: 1.5rem; height: 1.75rem; border-radius: .4rem; color: var(--accent); background: var(--accent-soft); }
.host-link__badge:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.host-link:last-child { border-bottom: 0; }
.host-link__row { display: flex; align-items: center; gap: 0.35rem; min-width: 0; }
.host-link__flags { display: flex; flex-wrap: wrap; gap: 0.15rem; max-width: 80px; flex-shrink: 0; }
.host-link__name { flex: 1; min-width: 0; overflow-wrap: anywhere; font-size: 0.85rem; font-weight: 600; }
.host-link__row :deep(button) { min-width: 44px; min-height: 44px; flex-shrink: 0; }
.host-link small { color: var(--text-muted); font-size: 0.75rem; }
.host-link__qr { display: grid; place-items: center; min-height: 280px; }
.host-link__qr img { max-width: 100%; height: auto; background: white; border-radius: 6px; }
.host-link__qr-loading { width: 280px; max-width: 100%; height: 280px; }
</style>
