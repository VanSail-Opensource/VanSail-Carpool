<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from '@/i18n'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import XNodeGlobalSettings from './XNodeGlobalSettings.vue'
import XNodeFirewall from './XNodeFirewall.vue'
import XNodeAutomaticLeases from './XNodeAutomaticLeases.vue'
import XNodeServiceUsers from './XNodeServiceUsers.vue'
import XNodeNodes from './XNodeNodes.vue'
import XNodeConfigurations from './XNodeConfigurations.vue'
import XNodeAccessGroups from './XNodeAccessGroups.vue'
import XNodeMetrics from './XNodeMetrics.vue'
import XNodeSubscriptionTemplates from './XNodeSubscriptionTemplates.vue'
import XNodeRoutingTemplates from './XNodeRoutingTemplates.vue'
import { useXNodeWorkspace } from './useXNodeWorkspace'
import { workspaceTab, workspaceTabs, workspaceNeedsMetrics } from './workspaceTabs'

const { t } = useI18n()
const route = useRoute(), router = useRouter()
const tab = computed({ get: () => workspaceTab(route.query.xnodeTab), set: (value: string | number) => { void router.replace({ query: { ...route.query, xnodeTab: workspaceTab(value) } }) } })
const workspace = useXNodeWorkspace(() => tab.value)
const tabs = computed(() => workspaceTabs.map(value => ({ value, disabled: workspace.busy.value, label: t(value === 'leases' ? 'automaticLeases.title' : value === 'routing' ? 'managedRouting.templatesTitle' : value === 'firewall' ? 'firewall.title' : value === 'templates' ? 'nodeTemplates.title' : `xnode.tabs.${value}`) })))
</script>

<template>
  <section class="xnode-workspace">
    <header class="admin-section-header"><div><h2>{{ t('xnode.title') }}</h2><p class="text-muted">{{ t('xnode.description') }}</p></div><UButton v-if="workspace.canRefresh.value" variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :disabled="workspace.busy.value" :loading="workspace.loading.value" @click="workspace.load" /></header>
    <div class="min-w-0 overflow-x-auto"><UTabs v-model="tab" :items="tabs" :content="false" variant="link" :ui="{ root: 'items-start', list: 'w-max min-w-full' }" /></div>
    <UAlert v-if="workspace.actionError.value" color="warning" :title="workspace.actionError.value" />
    <UAlert v-if="workspaceNeedsMetrics(tab) && workspace.metricError.value" color="warning" :title="workspace.metricError.value" />
    <AdminSectionState :key="tab" :loading="workspace.loading.value" :error="workspace.error.value" @retry="workspace.load">
      <XNodeNodes @refresh="workspace.load" v-if="tab === 'nodes'" :nodes="workspace.nodes.value" :endpoints="workspace.endpoints.value" :configurations="workspace.configurations.value" :metrics="workspace.metrics.value" :busy="workspace.busy.value" :perform="workspace.perform" :save-error="workspace.actionError.value" />
      <XNodeServiceUsers v-else-if="tab === 'services'" :nodes="workspace.nodes.value" :groups="workspace.groups.value" />
      <XNodeGlobalSettings v-else-if="tab === 'global'" />
      <XNodeFirewall v-else-if="tab === 'firewall'" />
      <XNodeAutomaticLeases v-else-if="tab === 'leases'" :nodes="workspace.nodes.value" />
      <XNodeSubscriptionTemplates v-else-if="tab === 'templates'" />
      <XNodeRoutingTemplates v-else-if="tab === 'routing'" />
      <XNodeConfigurations v-else-if="tab === 'configurations'" :configurations="workspace.configurations.value" :snippets="workspace.snippets.value" :busy="workspace.busy.value" :perform="workspace.perform" :save-error="workspace.actionError.value" />
      <XNodeAccessGroups v-else-if="tab === 'groups'" :configurations="workspace.configurations.value" :save-error="workspace.actionError.value" :groups="workspace.groups.value" :endpoints="workspace.endpoints.value" :nodes="workspace.nodes.value" :busy="workspace.busy.value" :perform="workspace.perform" @refresh="workspace.load" />
      <XNodeMetrics v-else-if="tab === 'metrics'" :metrics="workspace.metrics.value" :nodes="workspace.nodes.value" />
    </AdminSectionState>
  </section>
</template>

<style scoped>
.xnode-workspace { display: grid; grid-template-columns: minmax(0,1fr); gap: 20px; min-width: 0; }
.admin-section-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.admin-section-header h2 { margin: 0; font-size: 20px; }
.admin-section-header p { margin: 4px 0 0; font-size: 13px; }
</style>
