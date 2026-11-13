<script setup lang="ts">
import { watch } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import type { AffiliateOverview } from '@/api/features'
import { useClipboard } from '@/composables/useClipboard'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { formatMemberMoney } from '@/utils/displayCurrency'

const props = defineProps<{ overview: AffiliateOverview }>()
const clipboard = useClipboard()
const { reducedMotion } = useMotionPreferences()
watch(() => props.overview.inviteLink, clipboard.reset, { flush: 'sync' })
</script>

<template>
  <section class="affiliate-section affiliate-summary">
    <div class="affiliate-link-row">
      <div><span>{{ $t('affiliates.inviteLink') }}</span><strong>{{ overview.inviteLink ?? $t('affiliates.linkUnavailable') }}</strong></div>
      <UButton
        class="affiliate-copy-button" color="neutral" variant="outline" icon="i-ph-copy" :disabled="!overview.inviteLink"
        :label="clipboard.copied.value ? $t('common.copied') : $t('affiliates.copy')"
        data-haptic="copy"
        @click="overview.inviteLink && clipboard.copy(overview.inviteLink)"
      />
    </div>
    <dl class="affiliate-metrics">
      <div><dt>{{ $t('affiliates.totalCommission') }}</dt><dd><AnimatePresence mode="wait" :initial="false"><motion.output :key="formatMemberMoney(overview.totalCommission)" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }">{{ formatMemberMoney(overview.totalCommission) }}</motion.output></AnimatePresence></dd></div>
      <div><dt>{{ $t('affiliates.registered') }}</dt><dd><AnimatePresence mode="wait" :initial="false"><motion.output :key="overview.registeredCount" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }">{{ overview.registeredCount }}</motion.output></AnimatePresence></dd></div>
      <div><dt>{{ $t('affiliates.successful') }}</dt><dd><AnimatePresence mode="wait" :initial="false"><motion.output :key="overview.successfulCount" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }">{{ overview.successfulCount }}</motion.output></AnimatePresence></dd></div>
      <div><dt>{{ $t('affiliates.conversion') }}</dt><dd><AnimatePresence mode="wait" :initial="false"><motion.output :key="overview.conversionBps" :initial="reducedMotion ? { opacity: 0 } : { opacity: 0, y: 3 }" :animate="{ opacity: 1, y: 0 }" :exit="{ opacity: 0 }" :transition="{ duration: reducedMotion ? 0.08 : 0.2, ease: 'easeOut' }">{{ (overview.conversionBps / 100).toFixed(2) }}%</motion.output></AnimatePresence></dd></div>
    </dl>
  </section>
</template>
