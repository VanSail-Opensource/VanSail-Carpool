<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { DIYCompatibility } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import { selectionHaptic } from '@/utils/telegramHaptics'
import TuningCompatibilityList from './TuningCompatibilityList.vue'
const props = defineProps<{ items: DIYCompatibility[] }>()
const { t } = useI18n()
const format = shallowRef('xray')
const formats = computed(() => ['xray', 'sing-box', 'clash', 'stash', 'native', 'base64'].map(value => ({ value, label: t(`publicSubscription.formats.${value}`) })))
const selected = computed(() => props.items.filter(item => item.format === format.value).sort((a, b) => Number(a.state === 'inherited') - Number(b.state === 'inherited')))
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <UFormField :label="t('diy.clientFormat')"><USelect v-model="format" :items="formats" class="w-full" @update:model-value="selectionHaptic" /></UFormField>
    <TuningCompatibilityList :items="selected" />
  </section>
</template>
