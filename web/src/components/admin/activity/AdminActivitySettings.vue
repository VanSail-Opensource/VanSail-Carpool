<script setup lang="ts">
import { reactive, watch } from 'vue'

import type { ActivitySettings, ActivitySettingsWrite } from '@/api/features'
import { mergeRefreshedDraft } from '@/api/cache/drafts'
import TxbAmountField from '@/components/common/TxbAmountField.vue'
import { useI18n } from '@/i18n'
const props = defineProps<{ settings: ActivitySettings | null; busy: boolean; save: (value: ActivitySettingsWrite) => Promise<boolean> }>()
const draft = reactive({ timezone: 'Asia/Shanghai', dailyRewardMinTxb: '0.00', dailyRewardMaxTxb: '0.00', groupMessageThreshold: 0, groupMessageRewardTxb: '0.00' })
const { t } = useI18n()

watch(() => props.settings, (settings, previous) => {
  if (!settings) return
  const pick = (value: ActivitySettings) => ({
    timezone: value.timezone, dailyRewardMinTxb: value.dailyRewardMinTxb,
    dailyRewardMaxTxb: value.dailyRewardMaxTxb, groupMessageThreshold: value.groupMessageThreshold,
    groupMessageRewardTxb: value.groupMessageRewardTxb,
  })
  mergeRefreshedDraft(draft, pick(settings), previous ? pick(previous) : undefined)
}, { immediate: true })

function saveActivitySettings(): Promise<boolean> {
  return props.save({ ...draft })
}

defineExpose({ save: saveActivitySettings })
</script>

<template>
  <form class="activity-settings" @submit.prevent>
    <div>
      <h3>{{ t('adminActivity.dailyCheckIn') }}</h3>
      <p>{{ t('adminActivity.dailyCheckInHint') }}</p>
    </div>
    <UFormField name="activity-timezone" :label="t('adminActivity.calendarTimezone')" :hint="t('adminActivity.timezoneHint')" required><UInput v-model.trim="draft.timezone" class="w-full" :maxlength="80" /></UFormField>
    <TxbAmountField id="daily-reward-minimum" v-model="draft.dailyRewardMinTxb" :label="t('adminActivity.dailyRewardMinimum')" min-minor="0" required />
    <TxbAmountField id="daily-reward-maximum" v-model="draft.dailyRewardMaxTxb" :label="t('adminActivity.dailyRewardMaximum')" min-minor="0" required />
    <UFormField name="group-threshold" :label="t('adminActivity.groupMessageThreshold')" :hint="t('adminActivity.groupMessageThresholdHint')"><UInput v-model.number="draft.groupMessageThreshold" class="w-full" type="number" :min="0" :step="1" inputmode="numeric" /></UFormField>
    <TxbAmountField id="group-message-reward" v-model="draft.groupMessageRewardTxb" :label="t('adminActivity.groupMessageReward')" min-minor="0" required />
  </form>
</template>

<style scoped>
.activity-settings { display: grid; gap: 0.8rem; margin: 0 1rem 1rem; padding: 1rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface-raised); }
.activity-settings h3, .activity-settings p { margin: 0; }
.activity-settings p { margin-top: 0.25rem; color: var(--text-muted); font-size: 0.8rem; }
@media (min-width: 760px) { .activity-settings { grid-template-columns: minmax(180px, 0.8fr) repeat(2, minmax(190px, 1fr)); align-items: end; } }
</style>
