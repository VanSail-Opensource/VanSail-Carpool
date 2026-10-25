<script setup lang="ts">
import { watch } from 'vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'

const props = defineProps<{ error?: Error }>()
const emit = defineEmits<{ ready: [value: boolean] }>()
watch(() => props.error, error => emit('ready', Boolean(error)), { immediate: true })
function reload(): void { globalThis.location.reload() }
</script>

<template>
  <div v-if="error" role="alert" class="catalog-chunk-error">
    <InlineNotice tone="warning">{{ $t('recovery.description') }}</InlineNotice>
    <UButton color="neutral" variant="outline" :label="$t('recovery.reload')" @click="reload" />
  </div>
  <div v-else aria-busy="true"><SkeletonBlock height="12rem" /></div>
</template>

<style scoped>
.catalog-chunk-error { display: grid; gap: .6rem; justify-items: start; }
</style>
