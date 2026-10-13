<script setup lang="ts">
import { restoreItems } from '@/api/cache/restore'
import { mergeRefreshedDraft } from '@/api/cache/drafts'
import { computed, onMounted, reactive, shallowRef } from 'vue'

import { api, type AdminPaymentProfile } from '@/api/client'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import { localizedError, useI18n } from '@/i18n'

const { t } = useI18n()
const profiles = shallowRef<AdminPaymentProfile[]>([])
const loading = shallowRef(true)
const busy = shallowRef<string | null>(null)
const error = shallowRef<string | null>(null)
const success = shallowRef<string | null>(null)
const deleting = shallowRef<AdminPaymentProfile | null>(null)
const drafts = reactive<Record<string, AdminPaymentProfile>>({})
const channelIds = ['alipay', 'wxpay', 'qqpay', 'bank', 'jdpay']
const providerItems = computed(() => (['ezpay', 'bepusdt'] as const).map((provider) => ({
  label: t(`payment.providers.${provider}`),
  value: provider,
})))
const selectedId = shallowRef('')
const selected = computed(() => profiles.value.find(profile => profile.id === selectedId.value))
const columns = computed(() => [{ id: 'name', header: t('adminPaymentProfiles.providerName') }, { id: 'provider', header: t('adminPaymentProfiles.provider') }, { id: 'endpoint', header: t('adminPaymentProfiles.endpoint') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }])

function channelItems(): { label: string; value: string }[] {
  return channelIds.map((channel) => ({
    label: t(`adminPaymentProfiles.channelNames.${channel.replace('.', '_')}`),
    value: channel,
  }))
}

function emptyProfile(): AdminPaymentProfile {
  return {
    id: `draft-${Date.now()}`,
    provider: 'ezpay',
    providerName: '',
    enabledChannels: [],
    endpoint: '',
    merchantId: '',
    credential: '',
    acknowledgement: 'ok',
    enabled: false,
    configured: false,
  }
}

function addProfile(): void {
  const draft = emptyProfile()
  drafts[draft.id] = draft
  profiles.value = [...profiles.value, draft]
  selectedId.value = draft.id
}

function providerChanged(profile: AdminPaymentProfile): void {
  drafts[profile.id].enabledChannels = []
  drafts[profile.id].merchantId = ''
}

async function load(): Promise<void> {
  loading.value = !restoreItems('/api/v1/admin/payment-profiles', profiles)
  for (const profile of profiles.value) drafts[profile.id] = { ...profile, enabledChannels: [...profile.enabledChannels] }
  const previous = JSON.parse(JSON.stringify(drafts)) as typeof drafts
  try {
    const response = await api.getAdminPaymentProfiles()
    profiles.value = [...response.items, ...profiles.value.filter(profile => profile.id.startsWith('draft-'))]
    for (const profile of response.items) {
      if (!drafts[profile.id]) drafts[profile.id] = { ...profile, enabledChannels: [...profile.enabledChannels] }
      else mergeRefreshedDraft(drafts[profile.id], profile, previous[profile.id])
    }
  } catch (caught) { error.value = localizedError(caught, 'errors.adminLoad') } finally { loading.value = false }
}

async function save(profile: AdminPaymentProfile): Promise<void> {
  const draft = drafts[profile.id]
  if (!draft) return
  busy.value = profile.id
  error.value = null
  success.value = null
  try {
    const response = profile.id.startsWith('draft-')
      ? await api.createAdminPaymentProfile(draft)
      : await api.updateAdminPaymentProfile(profile.id, draft)
    if (selectedId.value === profile.id) selectedId.value = response.id
    delete drafts[profile.id]
    drafts[response.id] = { ...response, enabledChannels: [...response.enabledChannels] }
    profiles.value = profiles.value.map((item) => item.id === profile.id ? response : item)
    success.value = t('adminPaymentProfiles.saved')
  } catch (caught) { error.value = localizedError(caught, 'errors.adminAction') } finally { busy.value = null }
}

async function removeProfile(): Promise<void> {
  const profile = deleting.value
  if (!profile) return
  busy.value = profile.id
  error.value = null
  success.value = null
  try {
    if (!profile.id.startsWith('draft-')) await api.deleteAdminPaymentProfile(profile.id)
    profiles.value = profiles.value.filter((item) => item.id !== profile.id)
    delete drafts[profile.id]
    if (selectedId.value === profile.id) selectedId.value = ''
    deleting.value = null
    success.value = t('adminPaymentProfiles.deleted')
  } catch (caught) { error.value = localizedError(caught, 'errors.adminAction') } finally { busy.value = null }
}

async function saveAll(): Promise<void> {
  for (const profile of profiles.value) await save(profile)
}

defineExpose({ saveAll, loading })

onMounted(() => void load())
</script>

<template>
  <section class="payment-profiles">
    <div class="admin-panel__heading">
      <div><h2>{{ t('adminPaymentProfiles.title') }}</h2><p>{{ t('adminPaymentProfiles.copy') }}</p></div>
      <UButton icon="i-ph-plus" :label="t('adminPaymentProfiles.add')" @click="addProfile" />
    </div>
    <InlineNotice v-if="success" tone="success">{{ success }}</InlineNotice>
    <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
    <UTable class="admin-table" :data="profiles" :columns="columns" :loading="loading">
      <template #name-cell="{ row }">{{ drafts[row.original.id]?.providerName || t('adminPaymentProfiles.unnamed') }}</template>
      <template #provider-cell="{ row }">{{ t(`payment.providers.${drafts[row.original.id]?.provider}`) }}</template>
      <template #endpoint-cell="{ row }"><code>{{ drafts[row.original.id]?.endpoint }}</code></template>
      <template #status-cell="{ row }"><UBadge :color="drafts[row.original.id]?.enabled ? 'success' : 'neutral'" variant="subtle" :label="t(drafts[row.original.id]?.enabled ? 'common.enabled' : 'common.disabled')" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="selectedId = row.original.id" /><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminPaymentProfiles.delete')" :disabled="busy === row.original.id" @click="deleting = row.original" /></div></template>
    </UTable>
    <USlideover :open="!!selected" :title="selected ? drafts[selected.id]?.providerName || t('adminPaymentProfiles.unnamed') : t('adminPaymentProfiles.title')" :description="t('adminPaymentProfiles.copy')" :dismissible="!busy" @update:open="!$event && (selectedId = '')">
      <template #body>
        <div v-if="selected && drafts[selected.id]" class="admin-editor-form">
          <SwitchField :id="`payment-profile-${selected.id}`" v-model="drafts[selected.id].enabled" :label="t('common.enabled')" />
          <div v-if="selected.id.startsWith('draft-')"><UFormField :label="t('adminPaymentProfiles.provider')"><USelect v-model="drafts[selected.id].provider" :items="providerItems" @update:model-value="providerChanged(selected)" /></UFormField></div>
          <UFormField :label="t('adminPaymentProfiles.providerName')"><UInput v-model="drafts[selected.id].providerName" /></UFormField>
          <div v-if="drafts[selected.id].provider === 'ezpay'"><UFormField :label="t('adminPaymentProfiles.channels')" :description="t('adminPaymentProfiles.channelHint')"><UCheckboxGroup v-model="drafts[selected.id].enabledChannels" :items="channelItems()" orientation="vertical" variant="list" /></UFormField></div>
          <UAlert v-if="drafts[selected.id].provider === 'ezpay' && !drafts[selected.id].enabledChannels.length" color="warning" variant="soft" :description="t('adminPaymentProfiles.noChannels')" />
          <UAlert v-else-if="drafts[selected.id].provider === 'bepusdt'" color="neutral" variant="soft" icon="i-ph-arrows-clockwise" :description="t('adminPaymentProfiles.discoveryHint')" />
          <UFormField :label="t('adminPaymentProfiles.endpoint')"><UInput v-model="drafts[selected.id].endpoint" type="url" /></UFormField>
          <div v-if="drafts[selected.id].provider === 'ezpay'"><UFormField :label="t('adminPaymentProfiles.merchantId')"><UInput v-model="drafts[selected.id].merchantId" /></UFormField></div>
          <UFormField :label="t('adminPaymentProfiles.credential')"><UInput v-model="drafts[selected.id].credential" type="password" :placeholder="selected.configured ? t('adminPaymentProfiles.keepCredential') : ''" autocomplete="new-password" /></UFormField>
          <UFormField :label="t('adminPaymentProfiles.acknowledgement')"><UInput v-model="drafts[selected.id].acknowledgement" /></UFormField>
        </div>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      </template><template #footer><UButton color="neutral" variant="outline" :label="t('common.close')" @click="selectedId = ''" /><UButton v-if="selected" icon="i-ph-floppy-disk" :loading="busy === selected.id" :label="t('common.save')" @click="save(selected)" /></template>
    </USlideover>
    <ConfirmDialog
      :open="Boolean(deleting)"
      :title="t('adminPaymentProfiles.deleteTitle', { name: deleting?.providerName || t('adminPaymentProfiles.unnamed') })"
      :description="t('adminPaymentProfiles.deleteDescription')"
      :confirm-label="t('adminPaymentProfiles.deleteConfirm')"
      :busy="Boolean(deleting && busy === deleting.id)"
      danger
      @update:open="!$event && (deleting = null)"
      @confirm="removeProfile"
    />
  </section>
</template>

<style scoped>.payment-profiles { display: grid; gap: 1rem; }</style>
