<script setup lang="ts">
import { AnimatePresence, motion } from 'motion-v'

import type { StatisticsNodesSnapshot } from '@/api/types'
import CountryFlag from '@/components/common/CountryFlag.vue'
import { motionSpring } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { formatBytes, formatDateTime } from '@/utils/format'
import { formatStatisticNumber } from './statisticsFormat'
import { useI18n } from '@/i18n'

const props = defineProps<{
  snapshot: StatisticsNodesSnapshot | null
  loading: boolean
}>()

const { reducedMotion } = useMotionPreferences()
const { t } = useI18n()
const available = (node: StatisticsNodesSnapshot['nodes'][number]) => node.online && !props.snapshot?.stale
const rate = (node: StatisticsNodesSnapshot['nodes'][number], value: string) => available(node) ? t('statistics.perSecond', { value: formatBytes(value) }) : t('common.notAvailable')
</script>

<template>
  <section class="statistics-section">
    <div class="statistics-section__heading">
      <h2>{{ $t('statistics.nodes') }}</h2>
      <span v-if="snapshot" class="statistics-section__meta">{{ formatDateTime(snapshot.generatedAt) }}</span>
    </div>
    <div v-if="snapshot?.stale" class="statistics-stale-label"><UIcon name="i-ph-clock-counter-clockwise" aria-hidden="true" />{{ $t('statistics.lastKnown') }}</div>
    <div v-if="snapshot?.nodes.length" class="statistics-node-grid">
      <AnimatePresence :initial="false" mode="popLayout">
        <motion.article
          v-for="node in snapshot.nodes"
          :key="node.uuid"
          class="statistics-node"
          layout
          :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 5 }"
          :animate="{ opacity: 1, y: 0 }"
          :exit="{ opacity: 0, y: reducedMotion ? 0 : -4 }"
          :transition="reducedMotion ? { duration: 0.08 } : motionSpring"
        >
          <header>
            <CountryFlag :code="node.countryCode" />
            <UTooltip :text="node.name"><h3>{{ node.name }}</h3></UTooltip>
            <UTooltip :text="$t(available(node) ? 'statistics.online' : 'adminNative.unavailable')">
              <span
                class="statistics-node__state"
                :class="{ 'statistics-node__state--online': available(node) }"
                role="img"
                :aria-label="$t(available(node) ? 'statistics.online' : 'adminNative.unavailable')"
              >
                <UIcon :name="available(node) ? 'i-ph-wifi-high' : 'i-ph-wifi-slash'" aria-hidden="true" />
              </span>
            </UTooltip>
          </header>
          <dl>
            <div>
              <dt><UTooltip :text="$t('statistics.usersOnline')"><span class="statistics-node__metric-icon" role="img" :aria-label="$t('statistics.usersOnline')"><UIcon name="i-ph-users" aria-hidden="true" /></span></UTooltip></dt>
              <dd>{{ available(node) && node.usersOnlineAvailable ? formatStatisticNumber(node.usersOnline) : $t('common.notAvailable') }}</dd>
            </div>
            <div>
              <dt><UTooltip :text="$t('statistics.downloadRate')"><span class="statistics-node__metric-icon" role="img" :aria-label="$t('statistics.downloadRate')"><UIcon name="i-ph-arrow-down" aria-hidden="true" /></span></UTooltip></dt>
              <dd :title="rate(node, node.rxBytesPerSec)">{{ rate(node, node.rxBytesPerSec) }}</dd>
            </div>
            <div>
              <dt><UTooltip :text="$t('statistics.uploadRate')"><span class="statistics-node__metric-icon" role="img" :aria-label="$t('statistics.uploadRate')"><UIcon name="i-ph-arrow-up" aria-hidden="true" /></span></UTooltip></dt>
              <dd :title="rate(node, node.txBytesPerSec)">{{ rate(node, node.txBytesPerSec) }}</dd>
            </div>
          </dl>
        </motion.article>
      </AnimatePresence>
    </div>
    <div v-else class="statistics-empty statistics-empty--panel">
      <UIcon :name="loading ? 'i-ph-spinner-gap' : 'i-ph-broadcast'" :class="{ 'icon-spin': loading }" aria-hidden="true" />
      <span>{{ $t(loading ? 'statistics.loadingNodes' : 'statistics.noNodes') }}</span>
    </div>
  </section>
</template>
