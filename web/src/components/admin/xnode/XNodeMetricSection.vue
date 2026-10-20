<script setup lang="ts">
import { shallowRef } from 'vue'
import XNodeMetricDetails from './XNodeMetricDetails.vue'
defineProps<{ value: unknown; label: string; prefix?: string }>()
const open = shallowRef(false)
function toggle(event: globalThis.Event) { open.value = (event.currentTarget as InstanceType<typeof globalThis.HTMLDetailsElement>).open }
</script>
<template>
  <details @toggle="toggle"><summary class="cursor-pointer py-2">{{ label }}</summary><XNodeMetricDetails v-if="open" :value="value" :prefix="prefix" /></details>
</template>
