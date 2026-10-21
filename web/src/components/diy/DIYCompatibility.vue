<script setup lang="ts">
import { computed, shallowRef } from 'vue'
import type { DIYCompatibility } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import TuningCompatibilityList from './TuningCompatibilityList.vue'
const props = defineProps<{ items: DIYCompatibility[] }>()
const { t } = useI18n()
const format = shallowRef('xray')
const formats = computed(() => ['xray', 'sing-box', 'clash', 'stash', 'native', 'base64'].map(value => ({ value, label: t(`publicSubscription.formats.${value}`) })))
const selected = computed(() => props.items.filter(item => item.format === format.value))
const changed = computed(() => selected.value.filter(item => item.state !== 'inherited'))
const inherited = computed(() => selected.value.filter(item => item.state === 'inherited'))
const opened = shallowRef(false)
</script>
<template>
  <section class="grid gap-3 min-w-0">
    <h3 class="text-sm font-medium">{{ t('diy.compatibility') }}</h3>
    <UFormField :label="t('diy.clientFormat')"><USelect v-model="format" :items="formats" class="w-full" /></UFormField>
    <TuningCompatibilityList :items="changed" />
    <p class="text-xs text-muted">{{ t('diy.inheritedCount', { count: inherited.length }) }}</p>
    <details v-if="inherited.length" @toggle="opened = ($event.target as globalThis.HTMLDetailsElement).open"><summary class="text-xs cursor-pointer py-2">{{ t('diy.showInherited') }}</summary><TuningCompatibilityList v-if="opened" :items="inherited" /></details>
  </section>
</template>
