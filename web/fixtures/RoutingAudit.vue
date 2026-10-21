<script setup lang="ts">
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import XNodeRoutingTemplates from '@/components/admin/xnode/XNodeRoutingTemplates.vue'
import DIYCenter from '@/components/diy/DIYCenter.vue'
const admin = new globalThis.URLSearchParams(globalThis.location.search).has('admin')
const diy = new globalThis.URLSearchParams(globalThis.location.search).has('diy')
</script>
<template><UApp><main class="routing-audit-shell"><XNodeRoutingTemplates v-if="admin" /><DIYCenter v-else-if="diy" /><MemberRoutingEditor v-else /></main></UApp></template>
<style scoped>
.routing-audit-shell { max-width: 1120px; margin: 0 auto; padding: 24px; }
@media(max-width:640px) { .routing-audit-shell { padding: 16px; } }
</style>
