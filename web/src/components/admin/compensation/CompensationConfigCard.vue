<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { motion } from 'motion-v'

import type { NodeCompensationConfig, NodeCompensationConfigWrite } from '@/api/contracts/compensation'
import SwitchField from '@/components/common/SwitchField.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = defineProps<{ config: NodeCompensationConfig; busy: boolean }>()
const emit = defineEmits<{ save: [value: NodeCompensationConfigWrite] }>()
const { t } = useI18n()
const draft = reactive({ enabled: false, thresholdMinutes: 60, multiplier: 1 })
const { reducedMotion } = useMotionPreferences()

watch(() => props.config, (config) => {
  draft.enabled = config.enabled
  draft.thresholdMinutes = config.thresholdMinutes ?? 60
  draft.multiplier = (config.multiplierBps ?? 10_000) / 10_000
}, { immediate: true })

const valid = computed(() => Number.isInteger(draft.thresholdMinutes) && draft.thresholdMinutes >= 1 && draft.thresholdMinutes <= 5_256_000
  && draft.multiplier >= 0.01 && draft.multiplier <= 100)

function save(): void {
  if (!valid.value) return
  emit('save', {
    enabled: draft.enabled,
    thresholdMinutes: Math.trunc(draft.thresholdMinutes),
    multiplierBps: Math.round(draft.multiplier * 10_000),
    revision: props.config.revision,
  })
}
</script>

<template>
  <motion.form class="compensation-config" layout :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }" @submit.prevent="save">
    <div>
      <p class="eyebrow">{{ t('adminCompensation.configEyebrow') }}</p>
      <h3>{{ t('adminCompensation.configTitle') }}</h3>
      <p>{{ t('adminCompensation.configCopy') }}</p>
    </div>
    <SwitchField v-model="draft.enabled" id="compensation-enabled" :label="t('adminCompensation.enabled')" :help="t('adminCompensation.enabledHint')" />
    <div class="compensation-config__fields">
      <UFormField :label="t('adminCompensation.threshold')" required>
        <UInputNumber v-model="draft.thresholdMinutes" class="w-full" :min="1" :max="5256000" :step="1" />
      </UFormField>
      <UFormField :label="t('adminCompensation.multiplier')" :hint="t('adminCompensation.multiplierHint')" required>
        <UInputNumber v-model="draft.multiplier" class="w-full" :min="0.01" :max="100" :step="0.01" />
      </UFormField>
    </div>
    <UButton type="submit" icon="i-ph-floppy-disk" :label="busy ? t('common.working') : t('common.save')" :loading="busy" :disabled="busy || !valid" />
  </motion.form>
</template>

<style scoped>
.compensation-config { display: grid; grid-template-columns: minmax(0, 1fr); gap: 1rem; padding: 0; max-width: 48rem; }
.compensation-config h3, .compensation-config p { margin: 0; }
.compensation-config p:not(.eyebrow) { margin-top: 0.3rem; color: var(--text-muted); font-size: 0.8rem; line-height: 1.5; }
.compensation-config__fields { display: grid; grid-template-columns: minmax(0, 1fr); gap: 0.8rem; }
@media (min-width: 700px) { .compensation-config__fields { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
