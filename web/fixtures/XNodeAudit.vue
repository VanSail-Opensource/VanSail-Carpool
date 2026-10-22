<script setup lang="ts">
import FirewallExemption from '@/components/connections/FirewallExemption.vue'
const member = new globalThis.URLSearchParams(globalThis.location.search).has('firewall-member')
import XNodeWorkspace from '@/components/admin/xnode/XNodeWorkspace.vue'
import XNodeCommands from '@/components/admin/xnode/XNodeCommands.vue'
import AdminControlsAudit from './AdminControlsAudit.vue'
import CatalogQuotaAudit from './CatalogQuotaAudit.vue'
const quota = new globalThis.URLSearchParams(globalThis.location.search).has('combo-quota')
const commands = new globalThis.URLSearchParams(globalThis.location.search).has('commands')
const controls = new globalThis.URLSearchParams(globalThis.location.search).has('controls')
</script>

<template><UApp><main class="audit-shell"><CatalogQuotaAudit v-if="quota" /><AdminControlsAudit v-else-if="controls" /><XNodeCommands v-else-if="commands" node-id="node-one" :endpoints="[]" /><FirewallExemption v-else-if="member" /><XNodeWorkspace v-else /></main></UApp></template>

<style scoped>
.audit-shell { max-width: 1180px; padding: 24px; margin: 0 auto; }
@media(max-width:640px) { .audit-shell { padding: 16px; } }
</style>
