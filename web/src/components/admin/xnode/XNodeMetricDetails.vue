<script setup lang="ts">
import { computed } from 'vue'
import { metricRows } from './metricRows'

const props = defineProps<{ value: unknown; prefix?: string }>()
const rows = computed(() => metricRows(props.value, props.prefix))
</script>

<template>
  <dl class="metric-details"><div v-for="row in rows" :key="row.key"><dt>{{ row.label }}</dt><dd :title="row.raw">{{ row.value }}</dd></div></dl>
</template>

<style scoped>
.metric-details { display: grid; gap: 6px; min-width: 0; font-size: 12px; }
.metric-details div { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: 12px; }
dt { color: var(--text-muted); overflow-wrap: anywhere; }
dd { text-align: right; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
</style>
