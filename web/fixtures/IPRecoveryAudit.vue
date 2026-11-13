<script setup lang="ts">
import type { IPLookupBGPTopology, IPLookupLiveDetails, IPLookupReport } from '@/api/ipLookup'
import IPBGPTopology from '@/components/ip-lookup/IPBGPTopology.vue'
import IPLiveDetails from '@/components/ip-lookup/IPLiveDetails.vue'

defineProps<{ topology: IPLookupBGPTopology; report: IPLookupReport; snapshot: IPLookupLiveDetails; live: boolean }>()
</script>

<template>
  <UApp>
    <main class="ip-recovery-audit">
      <IPLiveDetails v-if="live" :report="report" :requested-ip="report.ip" :snapshot="snapshot" :refresh-quote="null" :busy="false" />
      <IPBGPTopology v-else :topology="topology" />
    </main>
  </UApp>
</template>

<style scoped>
.ip-recovery-audit { width: min(100%, 68rem); margin: auto; padding: 1rem; }
</style>
