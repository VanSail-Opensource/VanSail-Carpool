<script setup lang="ts">
import FirewallExemption from '@/components/connections/FirewallExemption.vue'
const member = new URLSearchParams(location.search).has('firewall-member')
import XNodeWorkspace from '@/components/admin/xnode/XNodeWorkspace.vue'
</script>

<template><UApp><main class="audit-shell"><FirewallExemption v-if="member" /><XNodeWorkspace v-else /></main></UApp></template>

<style scoped>
.audit-shell { max-width: 1180px; padding: 24px; margin: 0 auto; }
@media(max-width:640px) { .audit-shell { padding: 16px; } }
</style>
