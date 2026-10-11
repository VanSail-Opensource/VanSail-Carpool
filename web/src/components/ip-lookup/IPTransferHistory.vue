<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { IPLookupLiveDetails } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
const props = defineProps<{ history?: IPLookupLiveDetails['transfers']; loading: boolean; failed?: boolean }>()
const page = shallowRef(1)
const pages = computed(() => Math.max(1, Math.ceil((props.history?.transfers.length ?? 0) / 25)))
const visible = computed(() => props.history?.transfers.slice((page.value - 1) * 25, page.value * 25) ?? [])
watch(() => props.history, () => { page.value = 1 })
</script>

<template>
  <section class="transfer-history">
    <h3>{{ $t('ipLookupLive.transfers.title') }}</h3>
    <SkeletonBlock v-if="loading" height="3rem" />
    <p v-else-if="!history && failed">{{ $t('ipLookupLive.status.error') }}</p>
    <p v-else-if="!history">{{ $t('ipLookupLive.legacyData') }}</p>
    <template v-else>
      <p v-if="history.status === 'empty'">{{ $t('ipLookupLive.transfers.empty') }}</p>
      <ol v-if="visible.length">
        <li v-for="(transfer, index) in visible" :key="`${transfer.transferTime}:${transfer.recipientResource}:${index}`">
          <div><time :datetime="transfer.transferTime">{{ new Date(transfer.transferTime).toLocaleDateString() }}</time><code>{{ transfer.recipientResource }}</code></div>
          <dl><div><dt>{{ $t('ipLookupLive.transfers.from') }}</dt><dd>{{ transfer.sourceHolderName || $t('ipLookupLive.status.empty') }}<span v-if="transfer.sourceRegistry">{{ transfer.sourceRegistry }}</span><code v-if="transfer.sourceResource && transfer.sourceResource !== transfer.recipientResource">{{ transfer.sourceResource }}</code></dd></div><div><dt>{{ $t('ipLookupLive.transfers.to') }}</dt><dd>{{ transfer.recipientHolderName || $t('ipLookupLive.status.empty') }}<span v-if="transfer.recipientRegistry">{{ transfer.recipientRegistry }}</span></dd></div></dl>
        </li>
      </ol>
      <nav v-if="pages > 1" :aria-label="$t('ipLookupLive.transfers.title')"><UButton color="neutral" variant="ghost" icon="i-ph-caret-left" :disabled="page === 1" :aria-label="$t('ipLookupLive.transfers.previous')" @click="page--" /><span>{{ page }}/{{ pages }}</span><UButton color="neutral" variant="ghost" icon="i-ph-caret-right" :disabled="page === pages" :aria-label="$t('ipLookupLive.transfers.next')" @click="page++" /></nav>
      <LiveSourceNote :meta="history" />
    </template>
  </section>
</template>

<style scoped>
.transfer-history { border-top: 1px solid var(--line); padding-block: 0.8rem; }
.transfer-history h3 { font-size: 1rem; margin: 0; }
.transfer-history p, .transfer-history time, .transfer-history dt, .transfer-history nav { font-size: 0.75rem; color: var(--text-muted); }
.transfer-history ol { list-style: none; padding: 0; margin-block: 0.6rem; display: grid; gap: 0.8rem; }
.transfer-history li > div, .transfer-history dd { display: flex; flex-wrap: wrap; gap: 0.3rem 0.6rem; align-items: baseline; }
.transfer-history code, .transfer-history dd { font-size: 0.82rem; overflow-wrap: anywhere; }
.transfer-history dl { display: grid; gap: 0.4rem; margin: 0.4rem 0 0; }
.transfer-history dd { margin: 0.15rem 0 0; }
.transfer-history dd span { color: var(--text-muted); font-size: 0.75rem; }
.transfer-history nav { display: flex; align-items: center; justify-content: center; }
@media (min-width: 640px) { .transfer-history dl { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
