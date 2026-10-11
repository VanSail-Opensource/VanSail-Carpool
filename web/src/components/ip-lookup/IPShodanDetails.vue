<script setup lang="ts">
import { computed } from 'vue'
import type { IPLookupLiveDetails } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'

const props = defineProps<{ result: IPLookupLiveDetails['shodan'] }>()
const fields = ['hostnames', 'ports', 'tags', 'vulns', 'cpes'] as const
const rows = computed(() => fields.map(key => ({ key, items: props.result[key] ?? [], detected: key === 'tags' || key === 'vulns' })))
</script>

<template>
  <section class="shodan-details">
    <h3 class="shodan-title">{{ $t('ipLookupLive.shodanTitle') }}</h3>
    <dl class="shodan-facts">
      <div v-for="row in rows" :key="row.key" :class="{ 'shodan-cpes': row.key === 'cpes', 'shodan-detected': row.detected && row.items.length > 0 }">
        <dt>{{ $t(`ipLookupLive.shodanFields.${row.key}`) }}</dt>
        <dd>
          <ul v-if="row.items.length" class="shodan-items"><li v-for="(item, index) in row.items" :key="index"><code>{{ item }}</code></li></ul>
          <span v-else>{{ $t('ipLookupLive.status.empty') }}</span>
        </dd>
      </div>
    </dl>
    <LiveSourceNote :meta="result" />
    <details class="shodan-about"><summary>{{ $t('ipLookupLive.about') }}</summary><p>{{ $t('ipLookupLive.shodanContext') }}</p></details>
  </section>
</template>

<style scoped>
.shodan-details { min-width: 0; padding-block: 1rem; border-top: 1px solid var(--line); }
.shodan-title { font-size: 1rem; margin: 0; }
.shodan-facts { display: grid; gap: 0.9rem; margin-block: 0.75rem; }
.shodan-facts > div { min-width: 0; }
.shodan-facts dt, .shodan-about p { font-size: 0.78rem; color: var(--text-muted); }
.shodan-facts dd { margin: 0.25rem 0 0; font-size: 0.85rem; overflow-wrap: anywhere; }
.shodan-facts .shodan-detected dt, .shodan-detected dd { color: var(--ui-error); }
.shodan-items { display: flex; flex-wrap: wrap; gap: 0.25rem 0.8rem; margin: 0; padding: 0; list-style: none; }
.shodan-items li { min-width: 0; max-width: 100%; }
.shodan-cpes .shodan-items { display: grid; gap: 0.35rem; }
.shodan-about summary { color: var(--text-muted); font-size: 0.72rem; cursor: pointer; }
@media (min-width: 640px) { .shodan-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); } .shodan-cpes { grid-column: 1 / -1; } }
</style>
