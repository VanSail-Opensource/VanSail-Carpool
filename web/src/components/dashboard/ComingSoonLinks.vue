<script setup lang="ts">
import { computed } from 'vue'
import { useIPLookupAvailability } from '@/composables/useIPLookupAvailability'
import { useRouter } from 'vue-router'
import { usePreferencesStore } from '@/stores/preferences'

const props = defineProps<{ hasValidCombo: boolean }>()

const items = [
  { to: '/affiliates', labelKey: 'affiliates.title', noteKey: 'affiliates.dashboardNote', icon: 'i-ph-users-three' },
  { to: '/community', labelKey: 'community.title', noteKey: 'community.subtitle', icon: 'i-ph-users-three' },
  { to: '/emby', labelKey: 'nav.emby', noteKey: 'dashboard.embyNote', icon: 'i-ph-monitor-play' },
]

const { enabled: ipLookupEnabled } = useIPLookupAvailability()
const visibleItems = computed(() => [
  ...(ipLookupEnabled.value ? [{ to: '/ip-lookup', labelKey: 'ipLookup.title', noteKey: 'ipLookup.subtitle', icon: 'i-ph-globe-hemisphere-west' }] : []),
  ...(props.hasValidCombo && preferences.showAroundTX ? items : []),
])
const router = useRouter()
const preferences = usePreferencesStore()

function goTo(to: string): void {
  void router.push(to).catch(() => undefined)
}
</script>

<template>
  <section v-if="visibleItems.length" class="section-block home-around">
    <div class="section-heading">
      <h2>{{ $t('dashboard.aroundTx') }}</h2>
      <span class="section-heading__meta">{{ $t('dashboard.memberTools') }}</span>
    </div>
    <div class="home-around__links">
      <UButton
        v-for="item in visibleItems"
        :key="item.to"
        type="button"
        color="neutral"
        variant="ghost"
        class="home-around__link"
        data-haptic="navigate"
        @click="goTo(item.to)"
      >
        <span class="feature-icon feature-icon--small"><UIcon :name="item.icon" /></span>
        <span><strong>{{ $t(item.labelKey) }}</strong><small>{{ $t(item.noteKey) }}</small></span>
        <UIcon name="i-ph-arrow-right" />
      </UButton>
    </div>
  </section>
</template>

<style scoped>
@media (min-width: 900px) { .home-around { display: none; } }
</style>
