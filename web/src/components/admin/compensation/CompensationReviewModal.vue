<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { NodeCompensationEvent } from '@/api/contracts/compensation'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { eventExtension, multiplierFactor } from './format'

const open = defineModel<boolean>('open', { required: true })
const props = defineProps<{ event: NodeCompensationEvent | null; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ review: [action: 'approve' | 'dismiss', minutes: number, reason: string] }>()
const { t } = useI18n()
const minutes = ref(1)
const reason = ref('')

watch(() => props.event, (event) => {
  if (!event) return
  minutes.value = eventExtension(event)
  reason.value = ''
})
const ready = computed(() => reason.value.trim().length >= 3 && Number.isInteger(minutes.value)
  && minutes.value >= 1 && minutes.value <= 5_256_000)
const workflowState = computed(() => props.busy ? 'applying' : props.error ? `error:${props.error}` : props.event ? `review:${props.event.id}` : 'empty')
const { reducedMotion } = useMotionPreferences()
</script>

<template>
  <UModal v-model:open="open" :title="t('adminCompensation.reviewTitle')" :description="t('adminCompensation.reviewCopy')" :dismissible="!busy" :close="false" :ui="{ header: 'admin-overlay-header', wrapper: 'admin-overlay-copy', footer: 'justify-end flex-wrap' }">
    <template v-if="event" #body>
      <AnimatePresence mode="wait" :initial="false">
        <motion.div :key="workflowState" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
          <dl class="review-facts">
            <div><dt>{{ t('adminCompensation.node') }}</dt><dd>{{ event.nodeName }}</dd></div>
            <div><dt>{{ t('adminCompensation.squads') }}</dt><dd>{{ event.squads.map((squad) => squad.name).join(', ') || '—' }}</dd></div>
            <div><dt>{{ t('adminCompensation.frozenRecipients') }}</dt><dd>{{ event.frozenRecipientCount }}</dd></div>
            <div><dt>{{ t('adminCompensation.snapshotRule') }}</dt><dd>{{ event.thresholdMinutes }}m · {{ multiplierFactor(event.multiplierBps) }}×</dd></div>
          </dl>
          <UFormField :label="t('adminCompensation.extensionMinutes')" required>
            <UInputNumber v-model="minutes" class="w-full" :min="1" :max="5256000" :step="1" />
          </UFormField>
          <UFormField :label="t('adminCompensation.reviewReason')" required>
            <UTextarea v-model.trim="reason" :rows="3" :minlength="3" :maxlength="500" />
          </UFormField>
          <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        </motion.div>
      </AnimatePresence>
    </template>
    <template #footer>
      <UButton color="neutral" variant="outline" :label="t('adminCompensation.dismiss')" :disabled="busy || reason.trim().length < 3" @click="emit('review', 'dismiss', minutes, reason)" />
      <UButton color="warning" icon="i-ph-check" :label="t('adminCompensation.approve')" :loading="busy" :disabled="busy || !ready" @click="emit('review', 'approve', minutes, reason)" />
    </template>
  </UModal>
</template>

<style scoped>
.review-facts { display: grid; gap: 0.65rem; margin: 0 0 1rem; padding: 0.9rem; border: 1px solid var(--line); border-radius: var(--radius-control); }
.review-facts div { display: grid; grid-template-columns: minmax(7rem, 0.7fr) 1fr; gap: 0.7rem; }
.review-facts dt { color: var(--text-faint); font-size: 0.72rem; }
.review-facts dd { margin: 0; color: var(--text); font-size: 0.78rem; text-align: right; overflow-wrap: anywhere; }
</style>
