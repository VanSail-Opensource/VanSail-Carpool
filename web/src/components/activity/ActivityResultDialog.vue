<script setup lang="ts">
import { computed } from 'vue'

import type { ActivityResult } from '@/api/features'
import { useI18n } from '@/i18n'
import ActivityResultContent from './ActivityResultContent.vue'

const props = defineProps<{ result: ActivityResult | null }>()
defineEmits<{ close: [] }>()
const { t } = useI18n()

const description = computed(() => {
  const result = props.result
  if (!result) return ''
  if (result.kind === 'check_in') {
    const state = result.reward.kind === 'none' ? 'checkInRecorded' : 'checkInRewarded'
    return t('activity.resultDescription.' + state)
  }
  if (result.kind === 'bet') return t('activity.resultDescription.bet' + (result.outcome === 'win' ? 'Win' : 'Loss'))
  return t('activity.resultDescription.drawComplete')
})
</script>

<template>
  <UModal
    :open="Boolean(result)"
    :title="result ? $t('activity.result.' + result.outcome + '.title') : ''"
    :description="description"
    :close="false"
    :ui="{ header: 'tg-overlay-header--centered min-h-0 py-3 sm:py-3', wrapper: 'tg-overlay-copy--centered', ...(result?.kind === 'check_in' ? { body: 'p-3 sm:p-4', footer: 'p-3 sm:p-4' } : {}) }"
    @update:open="!$event && $emit('close')"
  >
    <template v-if="result" #body>
      <ActivityResultContent :result="result" />
    </template>
    <template #footer="{ close }">
      <UButton block :label="$t('common.close')" data-haptic="dismiss" @click="close" />
    </template>
  </UModal>
</template>
