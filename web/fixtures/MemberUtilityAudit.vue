<script setup lang="ts">
import { computed } from 'vue'
import { RouterView } from 'vue-router'
import { en, zh_cn } from '@nuxt/ui/locale'
import AppShell from '@/components/layout/AppShell.vue'
import AuthGate from '@/components/session/AuthGate.vue'
import SessionEntrance from '@/components/session/SessionEntrance.vue'
import type { SessionStatus } from '@/stores/session'
import { useI18n } from '@/i18n'

defineProps<{ page: string; phase: SessionStatus; message: string; code?: string }>()
defineEmits<{ retry: [] }>()
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>

<template>
  <UApp :locale="uiLocale">
    <SessionEntrance v-if="page === 'entry'" :status="phase" :authenticated="false">
      <AuthGate :message="message" :code="code" @retry="$emit('retry')" />
    </SessionEntrance>
    <AppShell v-else><RouterView /></AppShell>
  </UApp>
</template>
