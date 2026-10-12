<script setup lang="ts">
import type { IPLookupASNDetails, IPLookupReport } from '@/api/ipLookup'
import IPASNDetails from './IPASNDetails.vue'
import IPRPKIStatus from './IPRPKIStatus.vue'
defineProps<{ asns: IPLookupASNDetails[]; rpki: IPLookupReport['rpki'] }>()
</script>

<template>
  <section class="live-asn-section">
    <IPASNDetails v-if="asns.length" :asns="asns" :rpki="rpki" />
    <h3 v-else>{{ $t('ipLookupLive.asnTitle') }}</h3>
    <IPRPKIStatus v-if="!asns.length" :result="rpki" />
  </section>
</template>

<style scoped>
.live-asn-section { min-width: 0; }
.live-asn-section > h3 { margin: 0.75rem 0; font-size: 1rem; }
</style>
