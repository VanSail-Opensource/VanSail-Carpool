<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import ActivityPage from '@/components/activity/ActivityPage.vue'
import { useI18n } from '@/i18n'
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>
<template>
  <UApp :locale="uiLocale"><main id="main-content" style="max-width: 70rem; margin: 0 auto; padding: 1rem;"><ActivityPage /></main></UApp>
</template>
