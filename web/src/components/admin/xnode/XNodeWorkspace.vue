<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import XNodeGlobalSettings from './XNodeGlobalSettings.vue'
import XNodeFirewall from './XNodeFirewall.vue'
import XNodeServiceUsers from './XNodeServiceUsers.vue'
import XNodeNodes from './XNodeNodes.vue'
import XNodeConfigurations from './XNodeConfigurations.vue'
import XNodeAccessGroups from './XNodeAccessGroups.vue'
import XNodeMetrics from './XNodeMetrics.vue'
import XNodeMigration from './XNodeMigration.vue'
import XNodeSubscriptionTemplates from './XNodeSubscriptionTemplates.vue'
import { useXNodeWorkspace } from './useXNodeWorkspace'

const { t } = useI18n()
const workspace = useXNodeWorkspace()
const tab = shallowRef('nodes')
const tabs = computed(() => ['nodes', 'global', 'configurations', 'templates', 'services', 'groups', 'firewall', 'metrics', 'migration'].map(value => ({ value, label: t(value === 'firewall' ? 'firewall.title' : value === 'templates' ? 'nodeTemplates.title' : value === 'migration' ? 'xnodeMigration.title' : `xnode.tabs.${value}`) })))
</script>

<template>
  <section class="xnode-workspace">
    <header class="admin-section-header"><div><h2>{{ t('xnode.title') }}</h2><p class="text-muted">{{ t('xnode.description') }}</p></div><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="workspace.loading.value" @click="workspace.load" /></header>
    <div class="min-w-0 overflow-x-auto"><UTabs v-model="tab" :items="tabs" :content="false" variant="link" :ui="{ list: 'w-max min-w-full' }" /></div>
    <UAlert v-if="workspace.actionError.value" color="warning" :title="workspace.actionError.value" />
    <AdminSectionState :loading="workspace.loading.value" :error="workspace.error.value" @retry="workspace.load">
      <XNodeNodes v-if="tab === 'nodes'" :nodes="workspace.nodes.value" :endpoints="workspace.endpoints.value" :configurations="workspace.configurations.value" :metrics="workspace.metrics.value" :busy="workspace.busy.value" :perform="workspace.perform" :save-error="workspace.actionError.value" />
      <XNodeServiceUsers v-else-if="tab === 'services'" :nodes="workspace.nodes.value" :groups="workspace.groups.value" />
      <XNodeGlobalSettings v-else-if="tab === 'global'" />
      <XNodeFirewall v-else-if="tab === 'firewall'" />
      <XNodeSubscriptionTemplates v-else-if="tab === 'templates'" />
      <XNodeConfigurations v-else-if="tab === 'configurations'" :configurations="workspace.configurations.value" :snippets="workspace.snippets.value" :busy="workspace.busy.value" :perform="workspace.perform" :save-error="workspace.actionError.value" />
      <XNodeAccessGroups v-else-if="tab === 'groups'" :save-error="workspace.actionError.value" :groups="workspace.groups.value" :endpoints="workspace.endpoints.value" :nodes="workspace.nodes.value" :busy="workspace.busy.value" :perform="workspace.perform" />
      <XNodeMetrics v-else-if="tab === 'metrics'" :metrics="workspace.metrics.value" :nodes="workspace.nodes.value" />
      <XNodeMigration v-else :nodes="workspace.nodes.value" />
    </AdminSectionState>
  </section>
</template>

<style scoped>
.xnode-workspace { display: grid; grid-template-columns: minmax(0,1fr); gap: 20px; min-width: 0; }
.admin-section-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.admin-section-header h2 { margin: 0; font-size: 20px; }
.admin-section-header p { margin: 4px 0 0; font-size: 13px; }
</style>
