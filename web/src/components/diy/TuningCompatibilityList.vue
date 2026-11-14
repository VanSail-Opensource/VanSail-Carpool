<script setup lang="ts">
import type { DIYCompatibility } from '@/api/xnodeMember'
import type { DeepReadonly } from 'vue'
import { useI18n } from '@/i18n'
defineProps<{ items: DeepReadonly<DIYCompatibility[]> }>()
const { t } = useI18n()
const icons = { applied: 'i-ph-check-circle', inherited: 'i-ph-git-branch', unsupported: 'i-ph-minus-circle', 'endpoint-incompatible': 'i-ph-warning-circle', 'fixed-off': 'i-ph-lock-simple' }
</script>
<template>
  <ul class="compatibility-list">
    <li v-for="item in items" :key="`${item.format}:${item.key}`" class="compatibility-item">
      <div class="compatibility-option"><UIcon :name="icons[item.state]" class="compatibility-icon" :class="item.state === 'applied' ? 'text-success' : 'text-muted'" aria-hidden="true" /><span>{{ t(`diyFields.${item.key}`) }}</span></div>
      <span class="compatibility-state" :class="item.state === 'applied' ? 'text-success' : 'text-muted'">{{ t(`diy.states.${item.state}`) }}</span>
      <UPopover v-if="item.reasons.length || !['inherited', 'fixed-off'].includes(item.state) && item.restrictions?.length || ['applied', 'endpoint-incompatible'].includes(item.state)">
        <UButton icon="i-ph-info" size="sm" color="neutral" variant="ghost" :aria-label="t('diy.optionDetails', { name: t(`diyFields.${item.key}`) })" />
        <template #content>
          <div class="grid gap-2 max-w-xs p-3 text-xs">
            <p v-if="['applied', 'endpoint-incompatible'].includes(item.state)">{{ t('diy.hostCounts', { applied: item.appliedHosts, omitted: item.incompatibleHosts }) }}</p>
            <p v-for="reason in item.reasons" :key="reason">{{ t(`diy.reasons.${reason}`) }}</p>
            <p v-for="restriction in item.state === 'inherited' || item.state === 'fixed-off' ? [] : item.restrictions ?? []" :key="restriction">{{ t(`diy.restrictions.${restriction}`) }}</p>
          </div>
        </template>
      </UPopover>
      <span v-else aria-hidden="true" class="size-8" />
    </li>
  </ul>
</template>

<style scoped>
.compatibility-list { display: grid; min-width: 0; }
.compatibility-item { display: grid; grid-template-columns: minmax(0, 1fr) auto 2rem; align-items: center; gap: .65rem; padding-block: .6rem; border-bottom: 1px solid var(--line); font-size: .75rem; }
.compatibility-option { display: flex; align-items: flex-start; gap: .5rem; min-width: 0; overflow-wrap: anywhere; }
.compatibility-icon { flex: none; margin-top: .1rem; font-size: 1rem; }
.compatibility-state { max-width: 8.5rem; text-align: right; }
</style>
