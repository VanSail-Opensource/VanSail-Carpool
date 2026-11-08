<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useI18n } from '@/i18n'
import UsageBreakdownPanel from '@/components/dashboard/UsageBreakdownPanel.vue'
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>
<template>
  <UApp :locale="uiLocale"><main class="usage-audit"><UsageBreakdownPanel /></main></UApp>
</template>
<style scoped>
.usage-audit { width: min(100%, 860px); min-height: 100dvh; margin: 0 auto; padding: 1.25rem; }
</style>
