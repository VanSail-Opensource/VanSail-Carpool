<script setup lang="ts">
import { computed } from 'vue'
import type { DIYField, DIYOverrides } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import DIYSettingsGroup from './DIYSettingsGroup.vue'
import { diyFieldActive } from './diyApplicability'

const model = defineModel<DIYOverrides>({ required: true })
const props = defineProps<{ fields: DIYField[]; groups: string[]; errors: Record<string, string>; disabled?: boolean }>()
const { t } = useI18n()
const fields = computed(() => props.fields.filter(field => props.groups.includes(field.group) && diyFieldActive(field, model.value)))
</script>
<template>
  <p v-if="!fields.length" class="text-sm text-muted">{{ t('diy.noApplicableFields') }}</p>
  <DIYSettingsGroup v-else v-model="model" :fields="fields" :errors="errors" :disabled="disabled" />
</template>
