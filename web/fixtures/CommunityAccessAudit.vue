<script setup lang="ts">
import AppShell from '@/components/layout/AppShell.vue'
import { useCommunityAccess } from '@/composables/useCommunityAccess'
const access = useCommunityAccess()
</script>
<template>
  <AppShell>
    <h1>{{ $t('nav.home') }}</h1>
    <output data-test="community-access">{{ access.hasAccess.value }}</output>
    <output data-test="community-access-loading">{{ access.loading.value }}</output>
    <UButton data-test="refresh-access" :label="$t('common.refresh')" @click="access.refresh()" />
  </AppShell>
</template>
