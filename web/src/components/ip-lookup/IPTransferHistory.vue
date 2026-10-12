<script setup lang="ts">
import { computed, shallowRef, watch } from 'vue'
import type { IPLookupLiveDetails } from '@/api/ipLookup'
import LiveSourceNote from './LiveSourceNote.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
const props = defineProps<{ history?: IPLookupLiveDetails['transfers']; loading: boolean; failed?: boolean }>()
const page = shallowRef(1)
const transfers = computed(() => [...(props.history?.transfers ?? [])].sort((a, b) => (Date.parse(b.transferTime) || 0) - (Date.parse(a.transferTime) || 0)))
const pages = computed(() => Math.max(1, Math.ceil(transfers.value.length / 5)))
const visible = computed(() => transfers.value.slice((page.value - 1) * 5, page.value * 5))
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
          <div class="transfer-meta"><time :datetime="transfer.transferTime">{{ new Date(transfer.transferTime).toLocaleDateString() }}</time><code>{{ transfer.recipientResource }}</code></div>
          <div class="transfer-parties">
            <div><span class="sr-only">{{ $t('ipLookupLive.transfers.from') }}</span><span>{{ transfer.sourceHolderName || $t('ipLookupLive.status.empty') }}</span><small v-if="transfer.sourceRegistry">{{ transfer.sourceRegistry }}</small><code v-if="transfer.sourceResource && transfer.sourceResource !== transfer.recipientResource">{{ transfer.sourceResource }}</code></div>
            <UIcon name="i-ph-arrow-right" aria-hidden="true" />
            <div><span class="sr-only">{{ $t('ipLookupLive.transfers.to') }}</span><span>{{ transfer.recipientHolderName || $t('ipLookupLive.status.empty') }}</span><small v-if="transfer.recipientRegistry">{{ transfer.recipientRegistry }}</small></div>
          </div>
        </li>
      </ol>
      <nav v-if="pages > 1" :aria-label="$t('ipLookupLive.transfers.title')"><UButton color="neutral" variant="ghost" icon="i-ph-caret-left" :disabled="page === 1" :aria-label="$t('ipLookupLive.transfers.previous')" @click="page--" /><span>{{ page }}/{{ pages }}</span><UButton color="neutral" variant="ghost" icon="i-ph-caret-right" :disabled="page === pages" :aria-label="$t('ipLookupLive.transfers.next')" @click="page++" /></nav>
      <LiveSourceNote :meta="history" />
    </template>
  </section>
</template>

<style scoped>
.transfer-history { border-top: 1px solid var(--line); padding-block: 0.6rem; min-width: 0; }
.transfer-history h3 { font-size: 0.9rem; margin: 0; }
.transfer-history p, .transfer-history time, .transfer-history nav, .transfer-parties small { font-size: 0.72rem; color: var(--text-muted); }
.transfer-history ol { list-style: none; padding: 0; margin-block: 0.4rem; }
.transfer-history li { display: grid; gap: 0.3rem; padding-block: 0.4rem; }
.transfer-history li + li { border-top: 1px solid var(--line); }
.transfer-meta { display: flex; flex-wrap: wrap; gap: 0.25rem 0.6rem; align-items: baseline; }
.transfer-history code { font-size: 0.74rem; overflow-wrap: anywhere; }
.transfer-parties { display: grid; grid-template-columns: minmax(0, 1fr) 1rem minmax(0, 1fr); align-items: start; gap: 0.4rem; font-size: 0.8rem; }
.transfer-parties > div { min-width: 0; display: flex; flex-wrap: wrap; gap: 0.2rem 0.4rem; overflow-wrap: anywhere; }
.transfer-parties > .iconify { margin-top: 0.15rem; color: var(--text-muted); }
.transfer-history nav { display: flex; align-items: center; justify-content: center; }
@media (min-width: 640px) { .transfer-history li { grid-template-columns: minmax(0, 1fr) minmax(0, 3fr); gap: 0.7rem; } .transfer-meta { display: grid; align-content: start; } }
</style>
