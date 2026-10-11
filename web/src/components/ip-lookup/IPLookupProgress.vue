<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupProgress } from '@/api/ipLookup'
import { providerName } from './presentation'
import IPLookupProgressTasks from './IPLookupProgressTasks.vue'
const props = defineProps<{ progress: IPLookupProgress | null | undefined }>()
const value = computed(() => !props.progress?.total ? null : Math.min(props.progress.phase === 'complete' ? 100 : 99, Math.floor(100 * props.progress.completed / props.progress.total)))
</script>

<template>
  <div class="lookup-progress" role="status" aria-live="polite">
    <div><span>{{ progress?.currentProvider ? providerName(progress.currentProvider) : $t(`ipLookupProgress.${progress?.phase ?? 'queued'}`) }}</span><span v-if="value !== null">{{ value }}%</span></div>
    <UProgress :model-value="value" color="neutral" size="2xs" :aria-label="$t('ipLookupProgress.label')" />
    <IPLookupProgressTasks v-if="progress?.tasks?.length" :tasks="progress.tasks" />
  </div>
</template>

<style scoped>
.lookup-progress { display: grid; gap: 0.4rem; margin-block: 0.7rem; }
.lookup-progress > div { display: flex; justify-content: space-between; gap: 0.5rem; font-size: 0.75rem; color: var(--text-muted); }
</style>
