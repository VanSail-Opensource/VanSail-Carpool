<script setup lang="ts">
import { computed } from 'vue'
import { openExternalLink } from '@/utils/telegramLinks'
import { ipLocationMap } from './map'

const props = defineProps<{ latitude?: number | null; longitude?: number | null }>()
const map = computed(() => ipLocationMap(props.latitude, props.longitude))
</script>

<template>
  <figure v-if="map" class="location-map">
    <iframe :key="map.embed" :src="map.embed" :title="$t('ipLookup.mapTitle')" loading="lazy" referrerpolicy="no-referrer" sandbox="allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox" />
    <figcaption>
      <span>{{ $t('ipLookup.mapApproximate') }}</span>
      <UButton color="neutral" variant="link" icon="i-ph-arrow-square-out" :label="$t('ipLookup.openMap')" @click="openExternalLink(map!.link)" />
    </figcaption>
  </figure>
</template>

<style scoped>
.location-map { min-width: 0; margin: 0; display: flex; flex-direction: column; }
.location-map iframe { width: 100%; height: 220px; min-height: 220px; flex: 1; display: block; border: 0; border-radius: 12px; background: var(--surface); }
.location-map figcaption { display: flex; justify-content: space-between; align-items: center; gap: 0.5rem; color: var(--text-muted); font-size: 0.75rem; }
@media (min-width: 900px) { .location-map iframe { height: 280px; } }
</style>
