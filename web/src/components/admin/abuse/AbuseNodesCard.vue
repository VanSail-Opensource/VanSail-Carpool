<script setup lang="ts">
import { computed } from 'vue'
import type { AbuseNode } from '@/api/abuse'
import { useI18n } from '@/i18n'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ nodes: AbuseNode[]; statistics: Record<string, number>; busy: boolean }>()
const emit = defineEmits<{ copy: [id: string]; rotate: [id: string] }>()
const { t } = useI18n()
const statistics = computed(() => ['average', 'minimum', 'maximum'].map(name => ({ name, value: props.statistics[name] ?? 0 })))
</script>

<template>
  <section class="card nodes-card">
    <div>
      <p class="eyebrow">{{ t('adminAbuse.nodesEyebrow') }}</p>
      <h3>{{ t('adminAbuse.nodesTitle') }}</h3>
    </div>
    <div class="stats" :aria-label="t('adminAbuse.statisticsTitle')">
      <span v-for="item in statistics" :key="item.name">
        <small>{{ t(`adminAbuse.stat.${item.name}`) }}</small>
        <strong>{{ item.value }}</strong>
      </span>
    </div>
    <UTable class="admin-table" :data="nodes" :columns="[{ accessorKey: 'name', header: t('adminWorkspace.name') }, { accessorKey: 'uuid', header: t('adminWorkspace.identity') }, { id: 'reported', header: t('adminWorkspace.updated') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #reported-cell="{ row }">{{ row.original.lastReportAt ? formatDateTime(row.original.lastReportAt) : t('adminAbuse.neverReported') }}</template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-copy" :aria-label="t('adminAbuse.copyKey')" :disabled="busy" @click="emit('copy', row.original.uuid)" /><UButton color="neutral" variant="ghost" icon="i-ph-arrows-clockwise" :loading="busy" :aria-label="t('adminAbuse.rotateKey')" @click="emit('rotate', row.original.uuid)" /></div></template>
      <template #empty>{{ t('adminAbuse.emptyNodes') }}</template>
    </UTable>
  </section>
</template>

<style scoped>
.card { display: grid; gap: 0.85rem; padding: 0; }
.stats { display: grid; grid-template-columns: repeat(3, 1fr); gap: 0.6rem; }
.stats span { display: grid; gap: 0.15rem; }
.node { display: flex; min-height: 44px; align-items: center; justify-content: space-between; gap: 0.75rem; border-top: 1px solid var(--line); }
.node > div:first-child { display: grid; min-width: 0; gap: 0.2rem; }
.node strong { overflow-wrap: anywhere; }
.node small, .stats small { color: var(--text-muted); }
.node-actions { display: flex; gap: 0.35rem; }
</style>
