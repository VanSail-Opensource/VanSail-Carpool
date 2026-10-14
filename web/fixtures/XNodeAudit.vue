<script setup lang="ts">
import XNodeWorkspace from '@/components/admin/xnode/XNodeWorkspace.vue'
</script>

<template><UApp><main class="audit-shell"><XNodeWorkspace /></main></UApp></template>

<style scoped>
.audit-shell { max-width: 1180px; padding: 24px; margin: 0 auto; }
@media(max-width:640px) { .audit-shell { padding: 16px; } }
</style>
