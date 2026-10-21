<script setup lang="ts">
import { computed, type DeepReadonly } from 'vue'
import type { DIYCompatibility } from '@/api/xnodeMember'
import { useI18n } from '@/i18n'
import TuningCompatibilityList from '@/components/diy/TuningCompatibilityList.vue'
const props = defineProps<{ items?: DeepReadonly<DIYCompatibility[]>; format?: string }>()
const { t } = useI18n()
const omitted = computed(() => (props.items ?? []).filter(item => (!props.format || item.format === props.format) && (item.state === 'unsupported' || item.state === 'endpoint-incompatible' || item.incompatibleHosts > 0)))
</script>
<template><details v-if="omitted.length"><summary class="text-xs text-muted cursor-pointer py-2">{{ t('diy.compatibility') }}</summary><TuningCompatibilityList :items="omitted" /></details></template>
