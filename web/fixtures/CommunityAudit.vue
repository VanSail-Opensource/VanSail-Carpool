<script setup lang="ts">
import { en } from '@nuxt/ui/locale'
import CommunityPage from '@/components/community/CommunityPage.vue'
import CommunityAccessAudit from './CommunityAccessAudit.vue'
const access = new globalThis.URLSearchParams(globalThis.location.search).has('access')
</script>
<template><UApp :locale="en"><CommunityAccessAudit v-if="access" /><CommunityPage v-else /></UApp></template>
