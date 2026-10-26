<script setup lang="ts">
import { computed } from 'vue'

import type { Purchase } from '@/api/types'
import { useI18n } from '@/i18n'
import { formatBytes, formatDate } from '@/utils/format'

const props = defineProps<{
  active: Purchase
  squadNames?: readonly string[]
}>()
const emit = defineEmits<{ openPredictions: [] }>()

const { t } = useI18n()
const resetLabel = computed(() => t(`home.reset.${props.active.resetStrategy}`))
</script>

<template>
  <div class="home-ride__summary-face">
    <div class="home-ride__primary">
      <span class="feature-icon"><UIcon name="i-ph-stack" /></span>
      <div class="home-ride__name-block">
        <div class="home-ride__name-row">
          <h3>{{ active.comboName }}</h3>
        </div>
        <p>{{ squadNames?.length ? squadNames.join(t('home.squadSeparator')) : $t('dashboard.squadsIncluded', { count: active.squadUuids.length }) }}</p>
      </div>
      <UButton
        class="home-ride__flip-icon"
        color="neutral"
        variant="ghost"
        icon="i-ph-arrows-clockwise"
        :aria-label="$t('predictions.open')"
        data-haptic="open"
        @click="emit('openPredictions')"
      />
    </div>
    <dl class="home-ride__metrics">
      <div>
        <dt><UIcon name="i-ph-gauge" /> {{ $t('dashboard.traffic') }}</dt>
        <dd>{{ formatBytes(active.trafficLimitBytes) }}</dd>
      </div>
      <div>
        <dt><UIcon name="i-ph-arrow-clockwise" /> {{ $t('home.resetCadence') }}</dt>
        <dd>{{ resetLabel }}</dd>
      </div>
      <div>
        <dt><UIcon name="i-ph-calendar-blank" /> {{ $t('dashboard.renews') }}</dt>
        <dd>{{ formatDate(active.validUntil) }}</dd>
      </div>
    </dl>
    <span class="home-ride__flip-hint">{{ $t('predictions.open') }}</span>
  </div>
</template>

<style scoped>
.home-ride__name-block { min-width: 0; }
.home-ride__name-row { display: flex; align-items: center; gap: 0.55rem; min-width: 0; }
.home-ride__name-row h3 { min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
