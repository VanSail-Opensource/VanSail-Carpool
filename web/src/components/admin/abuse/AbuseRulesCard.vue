<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from 'vue'
import type { AbuseRule } from '@/api/abuse'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { useI18n } from '@/i18n'
import InlineNotice from '@/components/common/InlineNotice.vue'

const props = defineProps<{ rules: AbuseRule[]; whitelist: string[]; busy: boolean; error?: string | null }>()
const emit = defineEmits<{ saveRule: [value: AbuseRule]; deleteRule: [id: string, revision: number]; whitelist: [id: string, enabled: boolean] }>()
type FormError = { name?: string; message: string }
const { t } = useI18n()
const blankRule = (): AbuseRule => ({ id: '', name: '', expression: '', qpsLimit: 1, enabled: true, revision: 0 })
const draft = reactive<AbuseRule>(blankRule())
const whitelistForm = reactive({ remoteID: '' })
const editorOpen = shallowRef(false)
const selectedRuleID = shallowRef('')
const pendingRuleName = shallowRef('')
const deleting = shallowRef<Pick<AbuseRule, 'id' | 'name' | 'revision'> | null>(null)
const whitelistRows = computed(() => props.whitelist.map(id => ({ id })))

watch(() => props.rules, (rules) => {
  const selected = rules.find(item => item.id === selectedRuleID.value)
  if (selected) {
    Object.assign(draft, selected)
    return
  }
  if (!pendingRuleName.value) return
  const created = rules.find(item => item.name === pendingRuleName.value)
  if (!created) return
  selectedRuleID.value = created.id
  pendingRuleName.value = ''
  Object.assign(draft, created)
})

function selectRule(id: string): void {
  editorOpen.value = true
  pendingRuleName.value = ''
  selectedRuleID.value = id
  const rule = props.rules.find(item => item.id === id)
  if (rule) Object.assign(draft, rule)
}

function newRule(): void {
  editorOpen.value = true
  pendingRuleName.value = ''
  selectedRuleID.value = ''
  Object.assign(draft, blankRule())
}

function saveRule(): void {
  const value = { ...draft, name: draft.name.trim(), expression: draft.expression.trim() }
  pendingRuleName.value = value.id ? '' : value.name
  emit('saveRule', value)
}

function validateRule(value: Partial<AbuseRule>): FormError[] {
  const errors: FormError[] = []
  const { qpsLimit = 0 } = value
  if (!value.name?.trim() || value.name.trim().length > 120) errors.push({ name: 'name', message: t('adminAbuse.invalidRuleName') })
  if (!value.expression?.trim() || value.expression.trim().length > 1024) errors.push({ name: 'expression', message: t('adminAbuse.invalidExpression') })
  if (!Number.isInteger(qpsLimit) || qpsLimit < 1 || qpsLimit > 100000) errors.push({ name: 'qpsLimit', message: t('adminAbuse.invalidRuleLimit') })
  return errors
}

function validateWhitelist(value: { remoteID?: string }): FormError[] {
  return value.remoteID?.trim() && value.remoteID.trim().length <= 256
    ? []
    : [{ name: 'remoteID', message: t('adminAbuse.invalidWhitelistUser') }]
}

function addWhitelist(): void {
  const remoteID = whitelistForm.remoteID.trim()
  if (!remoteID) return
  emit('whitelist', remoteID, true)
  whitelistForm.remoteID = ''
}

function requestDelete(): void {
  if (draft.id) deleting.value = { id: draft.id, name: draft.name, revision: draft.revision }
}

function deleteRule(): void {
  if (!deleting.value) return
  emit('deleteRule', deleting.value.id, deleting.value.revision)
  deleting.value = null
}
</script>

<template>
  <section class="card rules-card">
    <div>
      <p class="eyebrow">{{ t('adminAbuse.rulesEyebrow') }}</p>
      <h3>{{ t('adminAbuse.rulesTitle') }}</h3>
    </div>
    <div class="admin-toolbar"><UButton icon="i-ph-plus" :label="t('adminAbuse.newRule')" @click="newRule" /></div>
    <UTable class="admin-table" :data="rules" :columns="[{ accessorKey: 'name', header: t('adminAbuse.ruleName') }, { accessorKey: 'expression', header: t('adminAbuse.expression') }, { accessorKey: 'qpsLimit', header: t('adminAbuse.ruleLimit') }, { id: 'status', header: t('adminWorkspace.status') }, { id: 'actions', header: t('adminWorkspace.actions') }]">
      <template #status-cell="{ row }"><UBadge :color="row.original.enabled ? 'success' : 'neutral'" variant="subtle" :label="t(row.original.enabled ? 'common.enabled' : 'common.disabled')" /></template>
      <template #actions-cell="{ row }"><div class="admin-row-actions"><UButton color="neutral" variant="ghost" icon="i-ph-pencil-simple" :aria-label="t('adminWorkspace.details')" @click="selectRule(row.original.id)" /><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminAbuse.deleteRule')" :disabled="busy" @click="deleting = { id: row.original.id, name: row.original.name, revision: row.original.revision }" /></div></template>
    </UTable>
    <USlideover v-model:open="editorOpen" :title="t('adminAbuse.rulesTitle')" :description="t('adminAbuse.rulesEyebrow')" :dismissible="!busy">
      <template #body>
        <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
        <UForm class="admin-editor-form" :state="draft" :validate="validateRule" @submit="saveRule">
          <UFormField name="name" :label="t('adminAbuse.ruleName')">
            <UInput v-model="draft.name" :maxlength="120" autocomplete="off" />
          </UFormField>
          <UFormField name="expression" :label="t('adminAbuse.expression')">
            <UInput v-model="draft.expression" :maxlength="1024" autocomplete="off" />
          </UFormField>
          <UFormField name="qpsLimit" :label="t('adminAbuse.ruleLimit')">
            <UInputNumber v-model="draft.qpsLimit" :min="1" :max="100000" :step="1" :disable-wheel-change="true" />
          </UFormField>
          <UFormField name="enabled" :label="t('adminAbuse.ruleEnabled')">
            <USwitch v-model="draft.enabled" />
          </UFormField>
          <div class="actions">
            <UButton type="submit" :loading="busy" :label="draft.id ? t('common.save') : t('adminAbuse.addRule')" />
            <UButton v-if="draft.id" type="button" color="neutral" variant="outline" :label="t('adminAbuse.newRule')" @click="newRule" />
            <UButton v-if="draft.id" type="button" color="error" variant="ghost" :label="t('adminAbuse.deleteRule')" @click="requestDelete" />
          </div>
        </UForm>
      </template>
    </USlideover>
    <div class="whitelist-heading">
      <h4>{{ t('adminAbuse.whitelistTitle') }}</h4>
      <p>{{ t('adminAbuse.whitelistCopy') }}</p>
    </div>
    <UForm :state="whitelistForm" :validate="validateWhitelist" class="whitelist-add" @submit="addWhitelist">
      <UFormField name="remoteID" :label="t('adminAbuse.whitelistUser')">
        <UInput v-model="whitelistForm.remoteID" :maxlength="256" autocomplete="off" />
      </UFormField>
      <UButton type="submit" :loading="busy" :label="t('adminAbuse.addWhitelist')" />
    </UForm>
    <UTable class="admin-table" :data="whitelistRows" :columns="[{ accessorKey: 'id', header: t('adminAbuse.whitelistUser') }, { id: 'actions', header: t('adminWorkspace.actions') }]"><template #actions-cell="{ row }"><UButton color="error" variant="ghost" icon="i-ph-trash" :aria-label="t('adminAbuse.removeWhitelist')" :disabled="busy" @click="emit('whitelist', row.original.id, false)" /></template><template #empty>{{ t('adminAbuse.emptyWhitelist') }}</template></UTable>
    <ConfirmDialog
      :open="Boolean(deleting)"
      :title="t('adminAbuse.deleteRuleTitle', { name: deleting?.name ?? '' })"
      :description="t('adminAbuse.deleteRuleDescription')"
      :confirm-label="t('adminAbuse.deleteRule')"
      :busy="busy"
      danger
      @update:open="!$event && (deleting = null)"
      @confirm="deleteRule"
    />
  </section>
</template>

<style scoped>
.card,
.card :deep(form) { display: grid; gap: 0.85rem; }
.card { padding: 0; }
.actions, .whitelist-add { display: flex; align-items: end; gap: 0.65rem; flex-wrap: wrap; }
.whitelist-add :deep(.form-field) { flex: 1 1 13rem; }
.whitelist-heading { padding-top: 0.25rem; border-top: 1px solid var(--line); }
.whitelist-heading p { color: var(--text-muted); }
.whitelist-item { display: flex; min-height: 44px; align-items: center; justify-content: space-between; gap: 0.65rem; border-top: 1px solid var(--line); overflow-wrap: anywhere; }
</style>
