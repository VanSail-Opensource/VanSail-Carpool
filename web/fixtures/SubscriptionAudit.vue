<script setup lang="ts">
import { computed } from 'vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import UserHome from '@/components/dashboard/UserHome.vue'
import SubscriptionPage from '@/components/subscription/SubscriptionPage.vue'
import AdminConnectivitySettings from '@/components/admin/connectivity/AdminConnectivitySettings.vue'
import { useI18n } from '@/i18n'
import HomeUptimePanel from '@/components/subscription/HomeUptimePanel.vue'
import MobileNavigation from '@/components/layout/MobileNavigation.vue'
import ComingSoonLinks from '@/components/dashboard/ComingSoonLinks.vue'
import { desktopNavigationItems } from '@/components/layout/navigation'
defineProps<{ mode: string }>()
const { locale, t } = useI18n()
const uiLocale = computed(() => locale.value === 'zh-CN' ? zh_cn : en)
</script>
<template><UApp :locale="uiLocale"><div v-if="mode === 'navigation'" class="p-4 grid gap-4"><UNavigationMenu orientation="vertical" :items="desktopNavigationItems(t, true, true, true)" /><ComingSoonLinks :has-valid-combo="true" /><MobileNavigation :is-admin="true" /></div><HomeUptimePanel v-else-if="mode === 'entrance'" /><UserHome v-else-if="mode === 'home'" /><AdminConnectivitySettings v-else-if="mode === 'admin'" /><SubscriptionPage v-else /></UApp></template>
