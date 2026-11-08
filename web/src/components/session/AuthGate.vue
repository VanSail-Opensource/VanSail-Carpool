<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'

import LanguageControl from '@/components/layout/LanguageControl.vue'
import { motionDurations } from '@/composables/motionPresets'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { isTelegramWebAppDetected } from '@/utils/telegram'
import { useI18n } from '@/i18n'

const props = defineProps<{ message: string; code?: string }>()
defineEmits<{ retry: [] }>()
const authRequestFailed = computed(() => isTelegramWebAppDetected() && props.message !== '')
const { t } = useI18n()
const heading = computed(() => props.code === 'ACCOUNT_RECOVERY_PENDING'
  ? t('auth.accountRecoveryPending')
  : t(authRequestFailed.value ? 'auth.authenticationFailed' : 'auth.openInTelegram'))
const { reducedMotion, offset } = useMotionPreferences()
</script>

<template>
  <motion.main
    class="auth-screen"
    :initial="reducedMotion ? { opacity: 1 } : { opacity: 0, y: offset(4) }"
    :animate="{ opacity: 1, y: 0 }"
    :transition="{ duration: reducedMotion ? 0 : motionDurations.fast, ease: 'easeOut' }"
  >
    <div class="auth-screen__copy">
      <p class="eyebrow">{{ $t('auth.telegramAccess') }}</p>
      <h1>{{ heading }}</h1>
      <p v-if="message !== heading">{{ message }}</p>
    </div>
    <UButton
      :label="$t('auth.tryAgain')"
      icon="i-ph-arrow-clockwise"
      data-haptic="retry"
      @click="$emit('retry')"
    />
    <footer class="auth-screen__locale"><LanguageControl /></footer>
  </motion.main>
</template>
