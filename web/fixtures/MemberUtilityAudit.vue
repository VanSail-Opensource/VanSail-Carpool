<script setup lang="ts">
import { computed } from 'vue'
import { RouterView } from 'vue-router'
import { en, zh_cn } from '@nuxt/ui/locale'
import AppShell from '@/components/layout/AppShell.vue'
import AuthGate from '@/components/session/AuthGate.vue'
import MemberSessionStatus from '@/components/session/MemberSessionStatus.vue'
import type { SessionStatus } from '@/stores/session'
import { useI18n } from '@/i18n'

defineProps<{ page: string; phase: SessionStatus; message: string; code?: string }>()
defineEmits<{ retry: [] }>()
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>

<template>
  <UApp :locale="uiLocale">
    <AuthGate v-if="page === 'entry' && phase === 'error'" :message="message" :code="code" @retry="$emit('retry')" />
    <AppShell v-else-if="page === 'entry'" :authorized="false">
      <template #pending><MemberSessionStatus :authenticated="false" /></template>
    </AppShell>
    <AppShell v-else><RouterView /></AppShell>
  </UApp>
</template>
