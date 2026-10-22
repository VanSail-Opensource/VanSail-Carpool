<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'

import InlineNotice from '@/components/common/InlineNotice.vue'
import LanguageControl from '@/components/layout/LanguageControl.vue'
import { useOnboarding } from '@/composables/useOnboarding'
import { useI18n } from '@/i18n'
import AgreementPanel from './AgreementPanel.vue'
import IntroSequence from './IntroSequence.vue'
import { useOnboardingMainButton } from './useOnboardingMainButton'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const {
  step,
  progress,
  loading,
  error,
  content,
  form,
  allAgreementsAccepted,
  finishIntro,
  acceptAgreement,
  toggleAgreement,
	loadContent,
} = useOnboarding()

const { t } = useI18n()
const mainAction = computed(() => {
  if (step.value === 'agreement') return {
    text: loading.value ? t('onboarding.finishing') : t('onboarding.finish'),
    disabled: !allAgreementsAccepted.value || loading.value,
    loading: loading.value,
    run: acceptAgreement,
  }
  return null
})
const { available: mainButtonAvailable } = useOnboardingMainButton(mainAction)
const { reducedMotion, offset } = useMotionPreferences()
const stepDirection = computed(() => step.value === 'agreement' ? 1 : -1)
</script>

<template>
  <IntroSequence v-if="step === 'intro' && content" :messages="content.welcome" @complete="finishIntro" />
  <main v-else class="onboarding-shell">
    <UProgress
      class="onboarding-shell__track"
      :aria-label="$t('onboarding.setupProgress')"
      :model-value="progress * 100"
      :max="100"
    />

    <div class="onboarding-shell__stage">
      <USkeleton v-if="loading && !content" class="h-48" />
      <AnimatePresence mode="wait" :initial="false">
        <motion.div
          v-if="step === 'agreement'"
          key="agreement"
          :initial="{ opacity: 0, y: offset(10) * stepDirection }"
          :animate="{ opacity: 1, y: 0 }"
          :exit="{ opacity: 0, y: reducedMotion ? 0 : -8 * stepDirection }"
          :transition="{ duration: reducedMotion ? 0.08 : motionDurations.step, ease: 'easeOut' }"
        >
          <AgreementPanel
            :agreements="content?.agreements ?? []"
            :selected-ids="form.agreementIds"
            :all-accepted="allAgreementsAccepted"
            :loading="loading"
            :show-action="!mainButtonAvailable"
            @toggle="toggleAgreement"
            @submit="acceptAgreement"
          />
        </motion.div>
      </AnimatePresence>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <UButton v-if="error && !content" color="neutral" variant="outline" :label="$t('common.tryAgain')" @click="loadContent" />
    </div>
    <footer class="onboarding-shell__locale"><LanguageControl /></footer>
  </main>
</template>
