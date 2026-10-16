<script setup lang="ts">
import { AnimatePresence, motion } from 'motion-v'

import type { CatalogNode } from '@/api/types'
import CountryFlag from '@/components/common/CountryFlag.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

defineProps<{ nodes: readonly CatalogNode[] }>()
const { reducedMotion } = useMotionPreferences()

function formatMultiplier(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 }).format(value)
}
</script>

<template>
  <div v-if="nodes.length" class="squad-node-list">
    <motion.div layout class="squad-node-list__grid">
      <AnimatePresence :initial="false" mode="popLayout">
        <motion.div v-for="node in nodes" :key="node.uuid" layout :initial="reducedMotion ? false : { opacity: 0, y: 4 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.14, ease: 'easeOut' }">
          <div
            class="squad-node-list__node"
          >
            <CountryFlag :code="node.countryCode" />
            <span class="squad-node-list__name">{{ node.name }}</span>
            <span class="squad-node-list__multiplier">{{ $t('catalog.nodeMultiplier', { multiplier: formatMultiplier(node.consumptionMultiplier) }) }}</span>
          </div>
        </motion.div>
      </AnimatePresence>
    </motion.div>
  </div>
</template>

<style scoped>
.squad-node-list { min-width: 0; display: grid; gap: 0.45rem; }
.squad-node-list__grid { min-width: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 13rem), 1fr)); gap: 0.45rem; }
.squad-node-list__grid > * { min-width: 0; }
.squad-node-list__node { width: 100%; min-width: 0; min-height: 44px; display: flex; align-items: center; gap: 0.5rem; padding: 0.45rem 0.55rem; border: 1px solid var(--line); border-radius: var(--radius-control); color: var(--text); background: var(--surface);  }
.squad-node-list__name { min-width: 0; flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; text-align: left; font-size: 0.75rem; }
.squad-node-list__node :deep(.country-flag) { width: 1.7rem; height: 1.2rem; flex: 0 0 auto; }
.squad-node-list__multiplier { color: var(--text-muted); font-family: var(--font-mono); font-size: 0.68rem; }
@media (min-width: 900px) {
  .squad-node-list__grid { grid-template-columns: repeat(auto-fit, minmax(min(100%, 8rem), 1fr)); gap: 0.35rem; }
  .squad-node-list__node { gap: 0.35rem; padding-inline: 0.4rem; }
  .squad-node-list__node :deep(.country-flag) { width: 1.5rem; height: 1.05rem; }
}
</style>
