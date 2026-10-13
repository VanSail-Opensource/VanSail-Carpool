<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useI18n } from '@/i18n'
import { useSessionStore } from '@/stores/session'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const sessionStore = useSessionStore()
const props = withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })

const groups = [
  { labelKey: 'adminNav.commerce', sections: [
    { value: 'usdt', labelKey: 'adminUsdt.title', icon: 'i-ph-currency-circle-dollar' },
    { value: 'catalog', labelKey: 'adminNav.catalog', icon: 'i-ph-package' },
    { value: 'coupons', labelKey: 'adminNav.coupons', icon: 'i-ph-ticket' },
  ] },
  { labelKey: 'adminNav.community', sections: [
    { value: 'activity', labelKey: 'adminNav.activity', icon: 'i-ph-game-controller' },
    { value: 'affiliates', labelKey: 'adminNav.affiliates', icon: 'i-ph-users-three' },
    { value: 'questionnaires', labelKey: 'adminNav.questionnaires', icon: 'i-ph-file-csv' },
    { value: 'onboarding', labelKey: 'adminNav.onboarding', icon: 'i-ph-user-plus-bold' },
  ] },
  { labelKey: 'adminNav.accounts', sections: [
    { value: 'users', labelKey: 'adminNav.users', icon: 'i-ph-user-focus' },
  ] },
  { labelKey: 'adminNav.system', sections: [
    { value: 'settings', labelKey: 'adminNav.settings', icon: 'i-ph-key' },
    { value: 'host-connectivity', labelKey: 'hostConnectivity.title', icon: 'i-ph-plugs-connected' },
    { value: 'compensation', labelKey: 'adminCompensation.nav', icon: 'i-ph-first-aid' },
    { value: 'abuse', labelKey: 'adminAbuse.nav', icon: 'i-ph-shield-warning' },
    { value: 'backups', labelKey: 'adminNav.backups', icon: 'i-ph-archive' },
    { value: 'database', labelKey: 'adminNav.database', icon: 'i-ph-database' },
    { value: 'audit', labelKey: 'adminNav.audit', icon: 'i-ph-shield-check' },
  ] },
] as const

const sectionItems = computed(() => groups.flatMap((group) => group.sections.map((section) => ({
  value: section.value,
  label: t('adminNav.sectionLabel', { group: t(group.labelKey), section: t(section.labelKey) }),
}))))
const selected = computed({
  get: () => String(route.params.section || route.meta.adminSection || 'settings'),
  set: (section: string) => void router.push(`/admin/${section}`),
})

function goToOnboarding(): void {
  void router.push('/onboarding')
}

</script>

<template>
  <div class="admin-shell" :class="{ 'admin-shell--compact': props.compact }">
    <header v-if="!props.compact" class="page-header admin-shell__header">
      <p class="eyebrow">{{ t('adminNav.eyebrow') }}</p>
      <h1>{{ t('adminNav.title') }}</h1>
      <p>{{ t('adminNav.copy') }}</p>
      <div v-if="!sessionStore.onboardingComplete" class="button-row">
        <UButton
          color="neutral"
          variant="outline"
          icon="i-ph-user-plus-bold"
          :label="t('adminNav.setupAccount')"
          @click="goToOnboarding"
        />
      </div>
    </header>
    <nav v-if="!props.compact" class="admin-section-picker" :aria-label="t('adminNav.sections')">
      <UFormField :label="t('adminNav.section')">
        <USelect v-model="selected" class="admin-section-picker__select" :items="sectionItems" value-key="value" />
      </UFormField>
    </nav>
    <div class="admin-shell__content"><slot /></div>
  </div>
</template>

<style scoped>
.admin-section-picker { max-width: 420px; margin: 0 0 0.9rem; padding: 0.75rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface-raised); }
.admin-section-picker__select { width: 100%; }
@media (min-width: 900px) { .admin-section-picker { display: none; } }
</style>
