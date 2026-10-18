<script setup lang="ts">
import { en } from '@nuxt/ui/locale'
import MemberLiveMetrics from '@/components/dashboard/MemberLiveMetrics.vue'
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import SubscriptionFormats from '@/components/subscription/SubscriptionFormats.vue'
const formatsOnly = new globalThis.URLSearchParams(globalThis.location.search).has('formats-only')
</script>

<template>
  <UApp :locale="en"><main class="page mx-auto max-w-5xl grid gap-4"><MemberLiveMetrics v-if="!formatsOnly" /><SubscriptionFormats /><MemberRoutingEditor v-if="!formatsOnly" /></main></UApp>
</template>
