<script setup lang="ts">
import { computed, type DeepReadonly } from 'vue'
import type { DIYCompatibility } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import TuningCompatibilityList from '@/components/diy/TuningCompatibilityList.vue'
const props = defineProps<{ items?: DeepReadonly<DIYCompatibility[]>; format?: string }>()
const { t } = useI18n()
const omitted = computed(() => (props.items ?? []).filter(item => (!props.format || item.format === props.format) && (item.state === 'unsupported' || item.state === 'endpoint-incompatible' || item.incompatibleHosts > 0)))
const clients = computed(() => {
  const groups = new Map<string, DeepReadonly<DIYCompatibility>[]>()
  for (const item of omitted.value) {
    if (!groups.has(item.format)) groups.set(item.format, [])
    groups.get(item.format)!.push(item)
  }
  return Array.from(groups, ([format, items]) => ({ format, items }))
})
</script>

<template>
  <section v-if="clients.length" class="client-compatibility">
    <h2 class="client-compatibility__title"><UIcon name="i-ph-devices" aria-hidden="true" />{{ t('diy.compatibility') }}</h2>
    <div class="client-compatibility__clients">
      <details v-for="client in clients" :key="client.format" class="client-compatibility__client" :open="!!format">
        <summary class="client-compatibility__summary">
          <UIcon name="i-ph-code" aria-hidden="true" />
          <span>{{ t(`publicSubscription.formats.${client.format}`) }}</span>
          <UIcon name="i-ph-caret-down" class="client-compatibility__chevron" aria-hidden="true" />
        </summary>
        <div class="client-compatibility__details"><TuningCompatibilityList :items="client.items" /></div>
      </details>
    </div>
  </section>
</template>

<style scoped>
.client-compatibility { display: grid; gap: .65rem; min-width: 0; }
.client-compatibility__title { display: flex; align-items: center; gap: .5rem; margin: 0; font-size: .85rem; font-weight: 600; }
.client-compatibility__title > span { color: var(--text-muted); }
.client-compatibility__clients { display: grid; gap: .4rem; }
.client-compatibility__client { min-width: 0; border: 1px solid var(--line); border-radius: var(--radius-control); }
.client-compatibility__summary { display: flex; align-items: center; gap: .5rem; min-height: 44px; padding: .55rem .7rem; cursor: pointer; list-style: none; font-size: .8rem; }
.client-compatibility__summary::-webkit-details-marker { display: none; }
.client-compatibility__summary:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; border-radius: inherit; }
.client-compatibility__chevron { margin-inline-start: auto; color: var(--text-muted); transition: transform 160ms ease; }
.client-compatibility__client[open] .client-compatibility__chevron { transform: rotate(180deg); }
.client-compatibility__details { padding: 0 .7rem .5rem; }
@media (prefers-reduced-motion: reduce) { .client-compatibility__chevron { transition: none; } }
</style>
