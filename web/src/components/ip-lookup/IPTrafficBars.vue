<script setup lang="ts">
import { getLocale } from '@/i18n'
import { computed } from 'vue'
import type { IPLookupTrafficSummary } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
const props = defineProps<{ summary: IPLookupTrafficSummary; title: string }>()
const names: Record<string, string> = { bot: 'bot', human: 'human', LIKELY_AUTOMATED: 'bot', LIKELY_HUMAN: 'human', desktop: 'desktop', mobile: 'mobile', other: 'other', DESKTOP: 'desktop', MOBILE: 'mobile', OTHER: 'other', IPv4: 'ipv4', IPv6: 'ipv6' }
const segments = computed(() => Object.entries(props.summary.values).filter(([name, value]) => names[name] && Number.isFinite(value) && value >= 0 && value <= 100).map(([name, value]) => ({ name: names[name]!, value })))
const total = computed(() => segments.value.reduce((sum, item) => sum + item.value, 0))
</script>

<template>
  <div class="traffic-summary">
    <h4>{{ title }}</h4>
    <div v-if="segments.length" class="traffic-track" role="img" :aria-label="segments.map(item => `${$t(`ipLookupLive.traffic.${item.name}`)} ${item.value.toFixed(1)}%`).join(', ')">
      <span v-for="item in segments" :key="item.name" :class="`traffic-${item.name}`" :style="{ width: `${total > 100 ? item.value / total * 100 : item.value}%` }" />
    </div>
    <ul v-if="segments.length" class="traffic-legend"><li v-for="item in segments" :key="item.name"><span :class="`traffic-${item.name}`" aria-hidden="true" />{{ $t(`ipLookupLive.traffic.${item.name}`) }}<strong>{{ item.value.toFixed(1) }}%</strong></li></ul>
    <p v-if="summary.startTime && summary.endTime" class="traffic-dates">{{ $t('ipLookupLive.traffic.range', { start: new Date(summary.startTime).toLocaleDateString(getLocale()), end: new Date(summary.endTime).toLocaleDateString(getLocale()) }) }}</p>
    <LiveSourceNote :meta="summary" />
  </div>
</template>

<style scoped>
.traffic-summary { min-width: 0; }
.traffic-summary h4 { margin: 0 0 0.6rem; font-size: 0.85rem; font-weight: 600; }
.traffic-track { display: flex; height: 0.6rem; overflow: hidden; border-radius: 3px; background: var(--surface-hover); }
.traffic-legend { display: flex; flex-wrap: wrap; gap: 0.5rem 0.9rem; padding: 0; margin: 0.65rem 0 0; list-style: none; font-size: 0.75rem; }
.traffic-legend li { display: inline-flex; align-items: center; gap: 0.35rem; }
.traffic-legend li > span { width: 0.45rem; height: 0.45rem; border-radius: 2px; }
.traffic-legend strong { font-variant-numeric: tabular-nums; font-weight: 500; }
.traffic-bot, .traffic-mobile, .traffic-ipv6 { background: var(--ui-info); }
.traffic-human, .traffic-desktop, .traffic-ipv4 { background: var(--ui-success); }
.traffic-other { background: var(--text-muted); }
.traffic-dates { margin: 0.45rem 0 0; font-size: 0.72rem; color: var(--text-muted); }
</style>
