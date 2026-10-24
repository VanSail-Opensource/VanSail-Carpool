<script setup lang="ts">
import type { AdminEntitlement } from '@/api/types'
import AdminEntitlementsPanel from '@/components/admin/AdminEntitlementsPanel.vue'
import AdminUserEntitlements from '@/components/admin/users/AdminUserEntitlements.vue'
import AdminUserProfilePage from '@/components/admin/users/AdminUserProfilePage.vue'
import EntitlementSummary from '@/components/dashboard/EntitlementSummary.vue'
import { useDashboard } from '@/composables/useDashboard'

defineProps<{ items: AdminEntitlement[]; inventory: boolean; profile: boolean }>()
const { dashboard } = useDashboard()
</script>

<template>
  <UApp>
    <main class="validity-audit">
      <AdminEntitlementsPanel v-if="inventory" />
      <AdminUserProfilePage v-else-if="profile" user-id="99101" />
      <template v-else>
        <EntitlementSummary :active="dashboard?.activePurchase" :queued="dashboard?.queuedPurchase" />
        <AdminUserEntitlements :items="items" :busy="false" />
      </template>
    </main>
  </UApp>
</template>

<style scoped>
.validity-audit { max-width: 1100px; margin: auto; padding: 16px; display: grid; gap: 24px; }
</style>
