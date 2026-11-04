<script setup lang="ts">
import { computed } from 'vue'
import { motion } from 'motion-v'

import type { DatabaseColumn, DatabaseValue } from '@/api/features'
import SwitchField from '@/components/common/SwitchField.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = defineProps<{
  insert?: boolean
  columns: readonly DatabaseColumn[]
  draft: Readonly<Record<string, DatabaseValue>>
  textDraft: Readonly<Record<string, string>>
  nullDraft: Readonly<Record<string, boolean>>
}>()
const emit = defineEmits<{
  'update:text': [column: string, value: string]
  'update:boolean': [column: string, value: boolean]
  'update:null': [column: string, value: boolean]
}>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const editableColumns = computed(() => props.columns.filter((column) => (props.insert ? column.insertable ?? column.editable : column.editable) && !column.sensitive))
const protectedColumns = computed(() => props.columns.filter((column) => !(props.insert ? column.insertable ?? column.editable : column.editable) || column.sensitive))

function isBoolean(column: DatabaseColumn): boolean {
  return typeof props.draft[column.name] === 'boolean' || /BOOL/i.test(column.declaredType)
}

function isBlob(column: DatabaseColumn): boolean {
  const value = props.draft[column.name]
  return (typeof value === 'object' && value !== null) || /BLOB|BINARY/i.test(column.declaredType)
}

function isNumeric(columnType: string): boolean {
  return /INT|REAL|FLOA|DOUB|NUMERIC|DECIMAL/i.test(columnType)
}

function protectedValue(column: DatabaseColumn): string {
  const value = props.draft[column.name]
  if (column.sensitive) return t('databaseRecord.masked')
  if (value === null) return t('adminDatabase.nullValue')
  if (typeof value === 'object') return t('adminDatabase.blobValue')
  return String(value ?? t('databaseRecord.notPresent'))
}
</script>

<template>
  <section class="database-record-fields" :aria-label="t('databaseRecord.editableFields')">
    <h3>{{ t('databaseRecord.editableFields') }}</h3>
    <p v-if="!editableColumns.length" class="database-record-fields__empty">{{ t('databaseRecord.noEditableFields') }}</p>
    <motion.div v-for="column in editableColumns" :key="column.name" class="database-record-fields__field" layout :transition="{ duration: reducedMotion ? 0.08 : 0.18, ease: 'easeOut' }">
      <SwitchField
        v-if="isBoolean(column) && !nullDraft[column.name]"
        :id="`db-${column.name}`"
        :model-value="draft[column.name] === true"
        :label="column.name"
        :help="column.declaredType"
        @update:model-value="emit('update:boolean', column.name, $event)"
      />
      <UFormField v-else :name="column.name" :label="column.name" :hint="column.declaredType">
        <UTextarea
          v-if="isBlob(column)"
          class="database-record-fields__control font-mono"
          :model-value="textDraft[column.name]"
          :rows="3"
          :disabled="nullDraft[column.name]"
          :placeholder="t('databaseRecord.base64')"
          @update:model-value="emit('update:text', column.name, String($event).trim())"
        />
        <UInput
          v-else
          class="database-record-fields__control font-mono"
          :model-value="textDraft[column.name]"
          :inputmode="isNumeric(column.declaredType) ? 'decimal' : 'text'"
          :disabled="nullDraft[column.name]"
          @update:model-value="emit('update:text', column.name, String($event))"
        />
      </UFormField>
      <UCheckbox
        v-if="column.nullable"
        :model-value="nullDraft[column.name]"
        :label="t('databaseRecord.storeNull')"
        @update:model-value="emit('update:null', column.name, Boolean($event))"
      />
    </motion.div>
    <details v-if="protectedColumns.length" class="database-record-fields__protected">
      <summary>{{ t('databaseRecord.protectedFields', { count: protectedColumns.length }) }}</summary>
      <dl>
        <div v-for="column in protectedColumns" :key="column.name">
          <dt><span>{{ column.name }}</span><small>{{ column.declaredType }}</small></dt>
          <dd><code>{{ protectedValue(column) }}</code><p v-if="column.protectionReason">{{ t(`databaseProtection.${column.protectionReason}`) }}</p></dd>
        </div>
      </dl>
    </details>
  </section>
</template>

<style scoped>
.database-record-fields { display: grid; min-width: 0; gap: 0.8rem; }
.database-record-fields h3 { margin: 0; font-size: 0.88rem; }
.database-record-fields__empty { margin: 0; color: var(--text-muted); font-size: 0.76rem; }
.database-record-fields__field { display: grid; min-width: 0; gap: 0.4rem; padding-bottom: 0.75rem; border-bottom: 1px solid var(--line); }
.database-record-fields__control :deep(input),
.database-record-fields__control :deep(textarea) { font-size: 1rem; }
.database-record-fields__protected { border: 1px solid var(--line); border-radius: var(--radius-control); background: var(--surface-raised); }
.database-record-fields__protected summary { min-height: 44px; display: flex; align-items: center; padding: 0.65rem 0.75rem; color: var(--text-muted); cursor: pointer; font-size: 0.75rem; font-weight: 700; }
.database-record-fields__protected dl { display: grid; gap: 0.55rem; margin: 0; padding: 0 0.75rem 0.75rem; }
.database-record-fields__protected dl > div { min-width: 0; display: grid; grid-template-columns: minmax(0, 0.45fr) minmax(0, 1fr); gap: 0.65rem; }
.database-record-fields__protected dt { min-width: 0; display: grid; gap: 0.1rem; overflow-wrap: anywhere; }
.database-record-fields__protected dt span { color: var(--text); font-size: 0.7rem; }
.database-record-fields__protected dt small { color: var(--text-faint); font-size: 0.62rem; }
.database-record-fields__protected dd { min-width: 0; margin: 0; color: var(--text-muted); font-size: 0.68rem; text-align: right; overflow-wrap: anywhere; }
.database-record-fields__protected code { font-family: var(--font-mono); }
</style>
