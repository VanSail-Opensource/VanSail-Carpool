<script setup lang="ts">
import type { DIYCompatibility } from '@/api/xnodeMember'
import type { DeepReadonly } from 'vue'
import { useI18n } from '@/i18n'
defineProps<{ items: DeepReadonly<DIYCompatibility[]> }>()
const { t } = useI18n()
</script>
<template>
  <ul class="grid gap-2 text-xs">
    <li v-for="item in items" :key="`${item.format}:${item.key}`" class="border-b border-default py-2">
      <div class="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-3"><span class="break-words">{{ t(`diyFields.${item.key}`) }}</span><span class="text-right" :class="item.state === 'applied' ? 'text-success' : 'text-muted'">{{ t(`diy.states.${item.state}`) }}</span></div>
      <p v-if="['applied', 'endpoint-incompatible'].includes(item.state)" class="text-muted">{{ t('diy.hostCounts', { applied: item.appliedHosts, omitted: item.incompatibleHosts }) }}</p>
      <p v-for="reason in item.reasons" :key="reason" class="text-muted">{{ t(`diy.reasons.${reason}`) }}</p>
      <p v-for="restriction in item.state === 'inherited' || item.state === 'fixed-off' ? [] : item.restrictions ?? []" :key="restriction" class="text-muted">{{ t(`diy.restrictions.${restriction}`) }}</p>
    </li>
  </ul>
</template>
