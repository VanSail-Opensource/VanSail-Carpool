<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { useI18n } from '@/i18n'
import AdminSectionState from '@/components/admin/AdminSectionState.vue'
import XNodeGlobalSettings from './XNodeGlobalSettings.vue'
import XNodeNodes from './XNodeNodes.vue'
import XNodeConfigurations from './XNodeConfigurations.vue'
import XNodeAccessGroups from './XNodeAccessGroups.vue'
import XNodeMetrics from './XNodeMetrics.vue'
import XNodeMigration from './XNodeMigration.vue'
import { useXNodeWorkspace } from './useXNodeWorkspace'

const { t } = useI18n()
const workspace = useXNodeWorkspace()
const tab = shallowRef('nodes')
const tabs = computed(() => ['nodes', 'global', 'configurations', 'groups', 'metrics', 'migration'].map(value => ({ value, label: t(value === 'migration' ? 'xnodeMigration.title' : `xnode.tabs.${value}`) })))
</script>

<template>
  <section class="xnode-workspace">
    <header class="admin-section-header"><div><h2>{{ t('xnode.title') }}</h2><p class="text-muted">{{ t('xnode.description') }}</p></div><UButton variant="ghost" color="neutral" icon="i-ph-arrow-clockwise" :aria-label="t('common.refresh')" :loading="workspace.loading.value" @click="workspace.load" /></header>
    <div class="min-w-0 overflow-x-auto"><UTabs v-model="tab" :items="tabs" :content="false" variant="link" :ui="{ list: 'w-max min-w-full' }" /></div>
    <AdminSectionState :loading="workspace.loading.value" :error="workspace.error.value" @retry="workspace.load">
      <XNodeNodes v-if="tab === 'nodes'" :nodes="workspace.nodes.value" :configurations="workspace.configurations.value" :metrics="workspace.metrics.value" :busy="workspace.busy.value" :perform="workspace.perform" />
      <XNodeGlobalSettings v-else-if="tab === 'global'" />
      <XNodeConfigurations v-else-if="tab === 'configurations'" :configurations="workspace.configurations.value" :snippets="workspace.snippets.value" :busy="workspace.busy.value" :perform="workspace.perform" />
      <XNodeAccessGroups v-else-if="tab === 'groups'" :groups="workspace.groups.value" :endpoints="workspace.endpoints.value" :nodes="workspace.nodes.value" :busy="workspace.busy.value" :perform="workspace.perform" />
      <XNodeMetrics v-else-if="tab === 'metrics'" :metrics="workspace.metrics.value" />
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
