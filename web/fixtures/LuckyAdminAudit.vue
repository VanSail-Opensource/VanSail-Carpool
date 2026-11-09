<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { useI18n } from '@/i18n'
import AdminActivityPanel from '@/components/admin/AdminActivityPanel.vue'
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>
<template>
  <UApp :locale="uiLocale"><main id="main-content" style="max-width: 74rem; margin: 0 auto; padding: 1rem;"><AdminActivityPanel /></main></UApp>
</template>
