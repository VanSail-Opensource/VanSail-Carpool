<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import AdminSettingsPanel from '@/components/admin/AdminSettingsPanel.vue'
import { useI18n } from '@/i18n'
const { locale } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>
<template><UApp :locale="uiLocale"><main class="browser-console"><AdminSettingsPanel /></main></UApp></template>
