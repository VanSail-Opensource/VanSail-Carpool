<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupReport } from '@/api/ipLookup'
import { refusalGroups } from './refusalPresentation'
const props = defineProps<{ refusals: IPLookupReport['refusals'] }>()
const groups = computed(() => refusalGroups(props.refusals))
</script>

<template>
  <div v-if="groups.length" class="refusals" :aria-label="$t('ipLookup.refusalHeading')">
    <details v-for="group in groups" :key="group.kind" :open="group.kind === 'shodan_tag' || group.kind === 'shodan_vuln'">
      <summary><UIcon :name="group.icon" aria-hidden="true" /><span>{{ $t(`ipLookup.refusals.${group.kind}`) }}</span><small v-if="group.items.length > 1">{{ group.items.length }}</small><UIcon name="i-ph-caret-down" aria-hidden="true" /></summary>
      <ul><li v-for="(item, index) in group.items" :key="index"><strong>{{ item.value === true ? $t('ipLookup.detected') : item.value === false ? $t('ipLookup.notDetected') : item.value }}</strong><span>{{ item.source }}</span></li></ul>
    </details>
  </div>
</template>

<style scoped>
.refusals { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.3rem 0.7rem; }
.refusals details { min-width: 0; }
.refusals summary { display: flex; align-items: center; gap: 0.35rem; min-height: 44px; font-size: 0.78rem; color: var(--ui-error); cursor: pointer; list-style: none; }
.refusals summary::-webkit-details-marker { display: none; }
.refusals summary > span { min-width: 0; overflow-wrap: anywhere; }
.refusals summary > :last-child { flex-shrink: 0; margin-left: auto; width: 0.75rem; }
.refusals details[open] summary > :last-child { transform: rotate(180deg); }
.refusals ul { list-style: none; margin: 0; padding: 0 0 0.5rem; }
.refusals li { display: grid; gap: 0.15rem; font-size: 0.75rem; overflow-wrap: anywhere; }
.refusals li + li { margin-top: 0.4rem; }
.refusals li span { color: var(--text-muted); font-size: 0.7rem; }
@media (min-width: 640px) { .refusals { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
