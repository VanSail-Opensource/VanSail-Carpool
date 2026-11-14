<script setup lang="ts">
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useHourly } from '@/components/hourly/useHourly'
import { useHourlyExtra } from '@/components/hourly/useHourlyExtra'
import { useI18n } from '@/i18n'
import { txbInputFromMinor } from '@/utils/format'
import SettingsSwitchRow from './SettingsSwitchRow.vue'

const { t } = useI18n()
const hourly = useHourly()
const extra = useHourlyExtra(hourly)
</script>

<template>
  <div v-if="!hourly.value.value?.legacyBlocked" class="settings-hourly" data-settings-hourly>
    <h3>{{ t('hourly.title') }}</h3>
    <USkeleton v-if="hourly.loading.value" class="h-16" />
    <SettingsSwitchRow v-else-if="hourly.value.value" :label="t('hourly.extraTitle')" :description="extra.unavailable.value ? t('hourly.extraUnavailable') : t('hourly.extraRate', { price: txbInputFromMinor(extra.price.value) })" :enabled="extra.enabled.value" :disabled="extra.disabled.value" :saving="hourly.submitting.value" @update="extra.request" />
    <UButton v-if="extra.priceChanged.value" data-hourly-price-review color="neutral" variant="link" icon="i-ph-currency-circle-dollar" :disabled="hourly.busy.value" :label="t('hourly.reviewPrice')" @click="extra.request(true)" />
    <InlineNotice v-if="hourly.error.value" tone="warning">{{ hourly.error.value }}<UButton variant="link" :label="t('common.tryAgain')" @click="hourly.refresh()" /></InlineNotice>
    <InlineNotice v-if="extra.error.value" tone="warning">{{ extra.error.value }}</InlineNotice>
    <ConfirmDialog v-model:open="extra.open.value" :title="t('hourly.extraTitle')" :description="t('hourly.extraWarning', { price: txbInputFromMinor(extra.price.value) })" :confirm-label="t('hourly.extraConfirm')" :busy="hourly.submitting.value" :error="extra.error.value" @confirm="extra.confirm" />
  </div>
</template>

<style scoped>
.settings-hourly { min-width: 0; padding-top: .75rem; }
.settings-hourly h3 { display: flex; align-items: center; gap: .5rem; margin: 0; color: var(--text-muted); font-size: .875rem; font-weight: 600; }
</style>
