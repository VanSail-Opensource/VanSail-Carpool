<script setup lang="ts">
import { restoreRef } from '@/api/cache/restore'
import { computed, onMounted, reactive, shallowRef, useTemplateRef, watch } from 'vue'
import type { AdminSetting } from '@/api/types'
import type { ActivitySettings, ActivitySettingsWrite } from '@/api/features'
import { featuresApi } from '@/api/features'
import AdminActivitySettings from '@/components/admin/activity/AdminActivitySettings.vue'
import AdminBillingAmountLimits from '@/components/admin/AdminBillingAmountLimits.vue'
import AdminPaymentProfiles from '@/components/admin/AdminPaymentProfiles.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SwitchField from '@/components/common/SwitchField.vue'
import { useAdminSection } from '@/composables/useAdminSection'
import { localizedError, useI18n } from '@/i18n'
import { writeAdminSettings } from '@/utils/adminSettingWrites'
import AdminSectionState from './AdminSectionState.vue'
const { items, loading, busy, error, load, perform } = useAdminSection<AdminSetting>('settings')
const draft = reactive<Record<string, string>>({})
const saved = reactive({ visible: false })
const activitySettings = shallowRef<ActivitySettings | null>(null)
const activityLoading = shallowRef(true)
const activityBusy = shallowRef(false)
const activityError = shallowRef<string | null>(null)
const savingAll = shallowRef(false)
type Saveable = { save: () => Promise<void>; loading: boolean }
type PaymentProfilesSaveable = { saveAll: () => Promise<void>; loading: boolean }
const activitySettingsRef = useTemplateRef<Saveable>('activitySettings')
const billingLimitsRef = useTemplateRef<Saveable>('billingLimits')
const paymentProfilesRef = useTemplateRef<PaymentProfilesSaveable>('paymentProfiles')
const { t } = useI18n()
const workspace = shallowRef('runtime')
const selectedCategory = shallowRef('')
const tabs = computed(() => [{ value: 'runtime', label: t('adminNav.settings') }, { value: 'payments', label: t('adminPaymentProfiles.title') }, { value: 'activity', label: t('adminNav.activity') }])
const activitySettingKeys = new Set([
  'activity.timezone', 'activity.daily_reward_min_txb', 'activity.daily_reward_max_txb',
  'activity.group_message_threshold', 'activity.group_message_reward_txb',
])
const clearableSettingKeys = new Set(['telegram.payment_announcement_chat_id', 'captcha.turnstile.site_key', 'telegram.pm.group_chat_id'])
const legacyPaymentSetting = (key: string): boolean => key.startsWith('billing.ezpay.') || key.startsWith('billing.bepusdt.') || key.startsWith('billing.usdt.')
const grouped = computed(() => items.value.filter((item) => !item.key.startsWith('ip_lookup.') && item.key !== 'connectivity.config' && !activitySettingKeys.has(item.key) && !legacyPaymentSetting(item.key)).reduce<Record<string, AdminSetting[]>>((groups, item) => {
  const category = item.category
  groups[category] ??= []
  groups[category].push(item)
  return groups
}, {}))
const categories = computed(() => Object.keys(grouped.value).map(value => ({ value, label: categoryLabel(value) })))
const category = computed({ get: () => selectedCategory.value || categories.value[0]?.value || '', set: value => { selectedCategory.value = value } })
const sectionsLoading = computed(() => loading.value
  || activityLoading.value
  || billingLimitsRef.value?.loading === true
  || paymentProfilesRef.value?.loading === true)
const saving = computed(() => savingAll.value || busy.value || activityBusy.value)
watch(items, (next, previous) => {
  for (const item of next) {
    const old = previous?.find(value => value.key === item.key)
    if (draft[item.key] === undefined || draft[item.key] === (old?.encrypted ? '' : old?.value)) {
      draft[item.key] = item.encrypted ? '' : item.value
    }
  }
}, { immediate: true })
function settingLabel(setting: AdminSetting): string {
  const key = `adminSettings.settingLabels.${setting.key.replace(/\./g, '_')}`
  const translated = t(key)
  if (translated !== key) return translated
  return setting.key
}
function categoryLabel(category: string): string {
  const key = `adminSettings.categories.${category.toLowerCase()}`
  const translated = t(key)
  return translated === key ? category : translated
}
function settingHelp(setting: AdminSetting): string {
  const key = `adminSettings.settingHints.${setting.key.replace(/\./g, '_')}`
  const translated = t(key)
  return translated === key ? t('adminSettings.validated') : translated
}
function isSensitive(setting: AdminSetting): boolean {
  return setting.encrypted
}
function isBoolean(setting: AdminSetting): boolean {
  return !setting.encrypted && (
    setting.value === 'true'
    || setting.value === 'false'
    || /(^|[._])(enabled|active|visible|allow|require|disabled)$/.test(setting.key)
  )
}
function setBoolean(key: string, value: boolean): void {
  draft[key] = value ? 'true' : 'false'
}
async function saveSettings(): Promise<void> {
  const values = Object.entries(draft).filter(([key, value]) => {
    const configured = items.value.some(item => item.key === key && item.configured)
    return key !== 'connectivity.config' && !activitySettingKeys.has(key) && !legacyPaymentSetting(key)
      && (value !== '' || (configured && clearableSettingKeys.has(key)))
  })
  const { api } = await import('@/api/client')
  saved.visible = await perform(() => writeAdminSettings(values, api.updateAdminSetting))
  if (saved.visible) {
    for (const [key, value] of values) {
      if (items.value.some(item => item.key === key && item.encrypted) && draft[key] === value) draft[key] = ''
    }
  }
}
async function saveAll(): Promise<void> {
  if (saving.value || sectionsLoading.value) return
  savingAll.value = true
  try {
    await Promise.all([
      saveSettings(),
      activitySettings.value ? activitySettingsRef.value?.save() ?? Promise.resolve() : Promise.resolve(),
      billingLimitsRef.value?.save() ?? Promise.resolve(),
      paymentProfilesRef.value?.saveAll() ?? Promise.resolve(),
    ])
  } finally {
    savingAll.value = false
  }
}
async function loadActivitySettings(): Promise<void> {
  activityLoading.value = !restoreRef('/api/v1/admin/activity-settings', activitySettings)
  activityError.value = null
  try {
    activitySettings.value = await featuresApi.getAdminActivitySettings()
  } catch (caught) {
    activityError.value = localizedError(caught, 'errors.adminLoad')
  } finally {
    activityLoading.value = false
  }
}
async function saveActivitySettings(value: ActivitySettingsWrite): Promise<void> {
  if (activityBusy.value) return
  activityBusy.value = true
  activityError.value = null
  try {
    activitySettings.value = await featuresApi.saveAdminActivitySettings(value)
  } catch (caught) {
    activityError.value = localizedError(caught, 'errors.adminAction')
  } finally {
    activityBusy.value = false
  }
}
onMounted(() => void loadActivitySettings())
</script>
<template>
  <section class="admin-panel">
    <div class="admin-panel__heading"><div><h2>{{ t('adminSettings.title') }}</h2><p>{{ t('adminSettings.copy') }}</p></div><UButton icon="i-ph-floppy-disk" :disabled="saving || sectionsLoading" :loading="saving" :label="saving ? t('common.saving') : t('adminSettings.save')" @click="saveAll" /></div>
    <UTabs v-model="workspace" :items="tabs" variant="link" :content="false" class="admin-workspace-tabs" />
    <InlineNotice v-if="saved.visible" tone="success" :title="t('adminSettings.saved')">{{ t('adminSettings.savedHint') }}</InlineNotice>
    <section v-show="workspace === 'activity'" class="settings-activity"><AdminActivitySettings ref="activitySettings" :settings="activitySettings" :busy="activityBusy" :save="saveActivitySettings" /><InlineNotice v-if="activityError" tone="warning">{{ activityError }}</InlineNotice></section>
    <div v-show="workspace === 'payments'" class="settings-payment-workspace"><AdminBillingAmountLimits ref="billingLimits" /><AdminPaymentProfiles ref="paymentProfiles" /></div>
    <div v-show="workspace === 'runtime'">
      <AdminSectionState :loading="loading" :error="error" @retry="load()">
        <div class="admin-toolbar"><USelect v-model="category" :items="categories" :aria-label="t('adminWorkspace.configuration')" /></div>
        <form class="settings-workspace" @submit.prevent="saveAll">
          <fieldset v-for="(settings, group) in grouped" v-show="category === group" :key="group" class="settings-workspace__fields">
            <legend>{{ categoryLabel(String(group)) }}</legend>
            <UFormField v-for="setting in settings" :key="setting.key" :name="setting.key" :label="settingLabel(setting)" :description="isSensitive(setting) ? t('adminSettings.secretHint') : settingHelp(setting)" :hint="setting.configured ? t('common.configured') : undefined">
              <SwitchField v-if="isBoolean(setting)" :id="`setting-${setting.key}`" :model-value="draft[setting.key] === 'true'" :label="t('common.enabled')" @update:model-value="setBoolean(setting.key, $event)" />
              <UInput v-else v-model="draft[setting.key]" :id="`setting-${setting.key}-input`" class="w-full" :name="setting.key" :aria-label="settingLabel(setting)" :icon="isSensitive(setting) ? 'i-ph-key' : undefined" :type="isSensitive(setting) ? 'password' : 'text'" :placeholder="isSensitive(setting) && setting.configured ? t('adminSettings.keepSecret') : ''" :autocomplete="isSensitive(setting) ? 'new-password' : 'off'" />
            </UFormField>
          </fieldset>
        </form>
      </AdminSectionState>
    </div>
  </section>
</template>
<style scoped>
.settings-workspace__fields { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1.35rem; border: 0; padding: 1.25rem 0; margin: 0; }
.settings-workspace__fields legend { color: var(--text-muted); font-size: .8rem; padding: 0; }
.settings-payment-workspace, .settings-activity { display: grid; gap: 1.5rem; }
@media (min-width: 1100px) { .settings-workspace__fields { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
