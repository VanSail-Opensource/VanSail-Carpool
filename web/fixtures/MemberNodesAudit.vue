<script setup lang="ts">
import { en } from '@nuxt/ui/locale'
import MemberLiveMetrics from '@/components/dashboard/MemberLiveMetrics.vue'
import MemberRoutingEditor from '@/components/subscription/MemberRoutingEditor.vue'
import SubscriptionFormats from '@/components/subscription/SubscriptionFormats.vue'
</script>

<template>
  <UApp :locale="en"><main class="page mx-auto max-w-5xl grid gap-4"><MemberLiveMetrics /><SubscriptionFormats /><MemberRoutingEditor /></main></UApp>
</template>
