<script setup lang="ts">
import type { IPLookupProgressTask } from '@/api/ipLookup'
import { taskLabel } from './presentation'

defineProps<{ tasks: IPLookupProgressTask[] }>()

const icons: Record<IPLookupProgressTask['status'], string> = {
  pending: 'i-ph-circle-dashed',
  active: 'i-ph-circle-notch',
  success: 'i-ph-check-circle',
  partial: 'i-ph-warning-circle',
  error: 'i-ph-x-circle',
  skipped: 'i-ph-minus-circle',
}
</script>

<template>
  <ul class="progress-tasks">
    <li v-for="task in tasks" :key="task.key" :class="`task-${task.status}`">
      <UIcon :name="icons[task.status] ?? 'i-ph-circle-dashed'" :class="{ spinning: task.status === 'active' }" aria-hidden="true" />
      <span>{{ taskLabel(task.key) }}</span>
      <span class="sr-only">{{ $t(`ipLookup.statuses.${task.status}`) }}</span>
    </li>
  </ul>
</template>

<style scoped>
.progress-tasks { display: grid; gap: 0.25rem 1rem; margin: 0; padding: 0; list-style: none; }
.progress-tasks li { display: flex; align-items: center; gap: 0.4rem; min-width: 0; color: var(--text-muted); font-size: 0.75rem; }
.progress-tasks li span:first-of-type { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.progress-tasks .spinning { animation: task-spin 1.2s linear infinite; }
.progress-tasks .task-partial, .progress-tasks .task-error { color: var(--warning); }
@keyframes task-spin { to { transform: rotate(360deg); } }
@media (min-width: 640px) { .progress-tasks { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
