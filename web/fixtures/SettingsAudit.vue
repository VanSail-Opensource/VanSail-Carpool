<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { en, zh_cn } from '@nuxt/ui/locale'
import SettingsPage from '@/components/settings/SettingsPage.vue'
import DIYView from '@/views/DIYView.vue'
import PurchaseActions from '@/components/dashboard/PurchaseActions.vue'
import type { Purchase } from '@/api/types'
import { useI18n } from '@/i18n'
const { locale } = useI18n()
const route = useRoute()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
defineProps<{ purchase: Purchase }>()
const refundMode = new globalThis.URLSearchParams(globalThis.location.search).has('refund')
</script>
<template><UApp :locale="uiLocale"><DIYView v-if="route.path === '/settings/diy'" /><PurchaseActions v-else-if="refundMode" :purchase="purchase" /><SettingsPage v-else /></UApp></template>
