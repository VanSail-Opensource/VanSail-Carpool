<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import { abuseApi } from '@/api/abuse'
import AbuseNodesCard from '@/components/admin/abuse/AbuseNodesCard.vue'
import AbusePolicyCard from '@/components/admin/abuse/AbusePolicyCard.vue'
import AbusePunishmentLadder from '@/components/admin/abuse/AbusePunishmentLadder.vue'
import AbuseRecordsCard from '@/components/admin/abuse/AbuseRecordsCard.vue'
import AbuseRulesCard from '@/components/admin/abuse/AbuseRulesCard.vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import { useAdminAbuse } from '@/composables/useAdminAbuse'
import { useClipboard } from '@/composables/useClipboard'
import { useI18n } from '@/i18n'
import AdminSectionState from './AdminSectionState.vue'

const { t } = useI18n()
const state = useAdminAbuse()
const workspace = shallowRef('policy')
const tabs = computed(() => [{ value: 'policy', label: t('adminAbuse.policyTitle') }, { value: 'punishments', label: t('adminAbuse.punishmentsTitle') }, { value: 'rules', label: t('adminAbuse.rulesTitle') }, { value: 'nodes', label: t('adminAbuse.nodesTitle') }, { value: 'records', label: t('adminAbuse.recordsTitle') }])
const clipboard = useClipboard()

function copy(id: string): void {
  void state.execute(async () => {
    const value = await abuseApi.copyNodeKey(id)
    await clipboard.copy(value.key)
  }, { errorKey: 'adminAbuse.copyFailed', reload: false })
}

function rotate(id: string): void {
  void state.execute(async () => {
    const value = await abuseApi.rotateNodeKey(id)
    await clipboard.copy(value.key)
  }, { errorKey: 'adminAbuse.rotateFailed' })
}

function savePolicy(value: Parameters<typeof abuseApi.savePolicy>[0]): void {
  void state.execute(async () => { await abuseApi.savePolicy(value) })
}

function savePunishment(value: Parameters<typeof abuseApi.savePunishment>[0]): void {
  void state.execute(async () => { await abuseApi.savePunishment(value) })
}

function deleteRule(id: string, revision: number): void {
  void state.execute(async () => { await abuseApi.deleteRule(id, revision) })
}

function setWhitelist(id: string, enabled: boolean): void {
  void state.execute(async () => { await abuseApi.setWhitelist(id, enabled) })
}
</script>

<template>
  <section class="admin-panel">
    <header class="admin-panel__heading">
      <div>
        <p class="eyebrow">{{ t('adminAbuse.eyebrow') }}</p>
        <h2>{{ t('adminAbuse.title') }}</h2>
        <p>{{ t('adminAbuse.copy') }}</p>
      </div>
      <UButton
        color="neutral"
        variant="outline"
        icon="i-ph-arrow-clockwise"
        :label="t('common.refresh')"
        :loading="state.loading.value"
        @click="state.load"
      />
    </header>

    <UTabs v-model="workspace" :items="tabs" :content="false" variant="link" class="admin-workspace-tabs" />
    <AdminSectionState :loading="state.loading.value" :error="state.loadError.value" @retry="state.load">
      <div v-if="state.policy.value" class="abuse-admin__grid">
        <InlineNotice v-if="state.operationError.value" class="abuse-admin__notice" tone="warning">
          {{ state.operationError.value }}
        </InlineNotice>
        <AbusePolicyCard
          v-show="workspace === 'policy'"
          :policy="state.policy.value"
          :busy="state.busy.value"
          @save="savePolicy"
        />
        <AbusePunishmentLadder
          v-show="workspace === 'punishments'"
          :punishments="state.punishments.value"
          :busy="state.busy.value"
          :error="state.operationError.value"
          @save="savePunishment"
        />
        <AbuseRulesCard
          v-show="workspace === 'rules'"
          :rules="state.rules.value"
          :whitelist="state.whitelist.value"
          :busy="state.busy.value"
          :error="state.operationError.value"
          @save-rule="state.saveRule"
          @delete-rule="deleteRule"
          @whitelist="setWhitelist"
        />
        <AbuseNodesCard v-show="workspace === 'nodes'" :nodes="state.nodes.value" :statistics="state.statistics.value" :busy="state.busy.value" @copy="copy" @rotate="rotate" />
        <AbuseRecordsCard
          v-show="workspace === 'records'"
          :records="state.records.value"
          :has-more="Boolean(state.nextRecordsCursor.value)"
          :loading-more="state.loadingMoreRecords.value"
          @load-more="state.loadMoreRecords"
        />
      </div>
    </AdminSectionState>
  </section>
</template>

<style scoped>
.abuse-admin__grid { display: grid; gap: 1rem; min-width: 0; }
</style>
