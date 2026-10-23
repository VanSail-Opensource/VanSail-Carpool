<script setup lang="ts">
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import XNodeRoutingTemplates from '@/components/admin/xnode/XNodeRoutingTemplates.vue'
import DIYCenter from '@/components/diy/DIYCenter.vue'
import MobileNavigation from '@/components/layout/MobileNavigation.vue'
const admin = new globalThis.URLSearchParams(globalThis.location.search).has('admin')
const diy = new globalThis.URLSearchParams(globalThis.location.search).has('diy')
const shell = new globalThis.URLSearchParams(globalThis.location.search).has('shell')
</script>
<template><UApp><div :class="{ 'app-frame': shell }"><main class="routing-audit-shell"><XNodeRoutingTemplates v-if="admin" /><DIYCenter v-else-if="diy" /><MemberRoutingEditor v-else /></main><MobileNavigation v-if="shell" :is-admin="false" /></div></UApp></template>
<style scoped>
.routing-audit-shell { max-width: 1120px; margin: 0 auto; padding: 24px; }
@media(max-width:640px) { .routing-audit-shell { padding: 16px; } }
</style>
