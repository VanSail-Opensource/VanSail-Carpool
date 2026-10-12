<script setup lang="ts">
import type { IPLookupReport } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
defineProps<{ result?: IPLookupReport['rpki']; compact?: boolean }>()
</script>

<template>
  <section class="rpki-status" :class="{ 'rpki-inline': compact }">
    <h3>{{ $t('ipLookupLive.rpki.title') }}</h3>
    <p v-if="!result">{{ $t('ipLookupLive.refreshRequired') }}</p>
    <template v-else>
      <code v-if="result.prefix">{{ result.prefix }}</code>
      <p v-if="result.status === 'empty'">{{ $t('ipLookupLive.rpki.notRouted') }}</p>
      <dl><div v-for="route in result.routes" :key="route.asn"><dt><code>AS{{ route.asn }}</code></dt><dd :class="{ 'rpki-invalid': route.validity === 'invalid_asn' || route.validity === 'invalid_length' }">{{ $t(`ipLookupLive.rpki.states.${route.validity}`) }}<div v-if="route.roas.length"><p v-for="(roa, index) in route.roas" :key="index"><code>AS{{ roa.asn }} {{ roa.prefix }}</code><span>{{ $t('ipLookupLive.rpki.maxLength', { length: roa.maxLength }) }}</span></p></div><LiveSourceNote v-if="route.status === 'error' && result.status === 'partial' && result.routes.length > 1" :meta="route" /></dd></div></dl>
      <LiveSourceNote :meta="result" />
    </template>
  </section>
</template>

<style scoped>
.rpki-status { border-top: 1px solid var(--line); padding-top: 0.6rem; font-size: 0.78rem; }
.rpki-inline { border-top: 0; padding-top: 0; }
.rpki-status h3 { font-size: 0.85rem; margin: 0 0 0.35rem; }
.rpki-status > p, .rpki-status span { color: var(--text-muted); }
.rpki-status dl { margin: 0.3rem 0; display: grid; gap: 0; }
.rpki-status dl > div { display: grid; grid-template-columns: 5rem minmax(0, 1fr); gap: 0.5rem; padding: 0.3rem 0; border-bottom: 1px solid var(--line); }
.rpki-status dd { margin: 0; display: flex; flex-wrap: wrap; align-items: baseline; gap: 0.3rem 0.6rem; }
.rpki-status .rpki-invalid { color: var(--ui-error); }
.rpki-status dd > div p { display: flex; flex-wrap: wrap; gap: 0.3rem 0.5rem; overflow-wrap: anywhere; }
</style>
