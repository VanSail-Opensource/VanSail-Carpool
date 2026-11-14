<script setup lang="ts">
import { computed, useId } from 'vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'
import { formatBytes, txbInputFromMinor } from '@/utils/format'

const props = withDefaults(defineProps<{
  rateBytesPerSecond: string
  extraAvailable: boolean
  extraPrice?: string
  busy?: boolean
  enabled?: boolean
  boosting?: boolean
}>(), { extraPrice: '0', busy: false, enabled: false, boosting: false })
defineEmits<{ extra: [enabled: boolean] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const switchId = useId()
const rateId = useId()
const speed = computed(() => t('hourly.rate', { rate: formatBytes(props.rateBytesPerSecond) }))
</script>

<template>
  <div class="hourly-throttle" :class="{ 'hourly-throttle--boosting': boosting, 'hourly-throttle--reduced': reducedMotion }" data-hourly-throttle>
    <div class="hourly-throttle__head" role="status">
      <span class="hourly-throttle__label"><UIcon name="i-ph-gauge" aria-hidden="true" />{{ t('hourly.throttledTitle') }}</span>
      <strong class="hourly-throttle__rate">{{ speed }}</strong>
    </div>
    <div class="hourly-throttle__visual" aria-hidden="true">
      <span class="hourly-throttle__endpoint"><UIcon name="i-ph-plugs-connected" /></span>
      <span class="hourly-throttle__track"><i /><i /><i /></span>
      <span class="hourly-throttle__endpoint hourly-throttle__destination"><UIcon :name="boosting ? 'i-ph-rocket-launch' : 'i-ph-gauge'" /></span>
    </div>
    <div v-if="extraAvailable" class="hourly-throttle__action">
      <label :for="switchId" class="hourly-throttle__copy">
        <span>{{ t('hourly.extraTitle') }}</span>
        <small :id="rateId">{{ t('hourly.extraRate', { price: txbInputFromMinor(extraPrice) }) }}</small>
      </label>
      <USwitch :id="switchId" :model-value="enabled" :loading="busy" :disabled="busy || boosting" :aria-label="t('hourly.extraTitle')" :aria-describedby="rateId" @update:model-value="$emit('extra', $event)" />
    </div>
    <p v-else class="hourly-throttle__hint">{{ t('hourly.throttledUnavailable') }}</p>
  </div>
</template>

<style scoped>
.hourly-throttle { --throttle-ink: var(--text-muted); display: grid; gap: .75rem; min-width: 0; padding: 1rem; border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); transition: border-color .3s, background-color .3s; }
.hourly-throttle__head { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .5rem; min-width: 0; }
.hourly-throttle__label { display: inline-flex; align-items: center; gap: .4rem; color: var(--text-muted); font-size: .75rem; }
.hourly-throttle__label .iconify { color: var(--warning); font-size: 1rem; }
.hourly-throttle__rate { color: var(--text); font: 600 .85rem var(--font-mono); white-space: nowrap; }
.hourly-throttle__visual { display: flex; align-items: center; gap: .6rem; color: var(--throttle-ink); }
.hourly-throttle__endpoint { display: grid; place-items: center; width: 2rem; height: 2rem; flex: none; border: 1px solid var(--line); border-radius: .5rem; font-size: 1.1rem; }
.hourly-throttle__track { position: relative; flex: 1; min-width: 0; height: 2px; overflow: hidden; background: var(--line); }
.hourly-throttle__track i { position: absolute; left: -12%; width: 12%; height: 2px; background: var(--throttle-ink); animation: throttle-packet 4.2s linear infinite; }
.hourly-throttle__track i:nth-child(2) { animation-delay: -1.4s; }
.hourly-throttle__track i:nth-child(3) { animation-delay: -2.8s; }
.hourly-throttle__destination { color: var(--warning); }
.hourly-throttle__action { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding-top: .7rem; border-top: 1px solid var(--line); }
.hourly-throttle__copy { display: grid; gap: .3rem; min-width: 0; color: var(--text); font-size: .8rem; font-weight: 500; line-height: 1.4; cursor: pointer; }
.hourly-throttle__copy small { color: var(--text-muted); font: .72rem var(--font-mono); }
.hourly-throttle__action :deep([role='switch']) { flex: none; position: relative; }
.hourly-throttle__action :deep([role='switch'])::before { content: ''; position: absolute; width: 44px; height: 44px; top: 50%; left: 50%; transform: translate(-50%, -50%); }
.hourly-throttle__hint { margin: 0; color: var(--text-muted); font-size: .75rem; line-height: 1.5; }
.hourly-throttle--boosting { --throttle-ink: var(--accent); border-color: color-mix(in srgb, var(--accent) 45%, var(--line)); }
.hourly-throttle--boosting .hourly-throttle__destination { color: var(--accent); border-color: var(--accent); }
.hourly-throttle--boosting .hourly-throttle__track i { animation-duration: .7s; }
.hourly-throttle--reduced, .hourly-throttle--reduced * { animation: none; transition: none; }
.hourly-throttle--reduced .hourly-throttle__track i { left: 40%; }
@keyframes throttle-packet { to { transform: translateX(1000%); } }
</style>
