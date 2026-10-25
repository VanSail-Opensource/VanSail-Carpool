<script setup lang="ts">
import { computed } from 'vue'

import type { GroupBoostStatus } from '@/api/features'
import { openExternalLink } from '@/utils/telegramLinks'

const props = defineProps<{ boost: GroupBoostStatus; refreshing: boolean }>()
defineEmits<{ refresh: [] }>()
const boosted = computed(() => props.boost.state === 'boosted')
const unavailable = computed(() => props.boost.state === 'unavailable')
const multiplier = computed(() => props.boost.count ? Math.max(1, props.boost.count / 2) : 0)

function openBoost(): void {
  if (props.boost.boostUrl) openExternalLink(props.boost.boostUrl)
}
</script>

<template>
  <section class="section-block boost-panel" :class="{ 'boost-panel--active': boosted }">
    <div class="boost-panel__copy">
      <h2>{{ $t(boosted ? 'activity.boostActive' : unavailable ? 'activity.boostUnavailable' : 'activity.boostRequired') }}</h2>
      <p v-if="boosted">{{ $t('activity.boostCount', { count: boost.count ?? 0, multiplier }) }}</p>
      <template v-else>
        <p>{{ $t(unavailable ? 'activity.boostUnavailableHint' : 'activity.boostRequiredHint') }}</p>
        <p class="boost-panel__rule">{{ $t('activity.boostMultiplierHint') }}</p>
      </template>
    </div>
    <div class="boost-panel__actions">
      <UButton v-if="!boosted && boost.boostUrl" color="neutral" :label="$t('activity.boostGroup')" icon="i-ph-lightning-fill" @click="openBoost" />
      <UButton
        :label="$t('activity.boostCheckAgain')"
        variant="ghost" color="neutral"
        :loading="refreshing" :disabled="refreshing"
        @click="$emit('refresh')"
      />
    </div>
  </section>
</template>

<style scoped>
.boost-panel { display: grid; gap: .65rem; }
.boost-panel__copy h2 { margin: 0; font-size: 1.05rem; }
.boost-panel__copy p { margin: 0.4rem 0 0; color: var(--text-muted); font-size: 0.82rem; }
.boost-panel__copy .boost-panel__rule { color: var(--text-faint); }
.boost-panel__actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
.boost-panel__actions :deep(button) { flex: 1; }
@media (min-width: 640px) {
  .boost-panel--active { grid-template-columns: minmax(0, 1fr) auto; align-items: center; }
  .boost-panel__actions :deep(button) { flex: none; }
}
</style>
