<script setup lang="ts">
import { useI18n } from '@/i18n'

withDefaults(defineProps<{
  variant?: 'list' | 'form' | 'chart' | 'compact'
  rows?: number
  label?: string
}>(), { variant: 'list', rows: 3, label: undefined })

const { t } = useI18n()
</script>

<template>
  <div class="loading-placeholder min-w-0" :data-variant="variant" role="status" aria-live="polite" aria-atomic="true">
    <span class="sr-only">{{ label ?? t('common.loading') }}</span>
    <div v-if="variant === 'form'" class="grid gap-5" aria-hidden="true">
      <div class="grid gap-x-5 gap-y-4 sm:grid-cols-2">
        <div v-for="row in rows" :key="row" class="grid min-w-0 gap-2">
          <USkeleton class="h-3 w-2/5 rounded-full" />
          <USkeleton class="h-11 w-full rounded-xl" />
        </div>
      </div>
      <div class="flex justify-end gap-2">
        <USkeleton class="h-11 w-24 rounded-xl" />
        <USkeleton class="h-11 w-24 rounded-xl" />
      </div>
    </div>
    <div v-else-if="variant === 'chart'" class="flex flex-wrap items-center justify-center gap-6 py-4" aria-hidden="true">
      <USkeleton class="size-40 max-w-full shrink-0 rounded-full" />
      <div class="grid min-w-0 flex-1 gap-4">
        <div v-for="row in rows" :key="row" class="flex items-center gap-3">
          <USkeleton class="size-3 shrink-0 rounded-full" />
          <USkeleton class="h-3 rounded-full" :class="row % 2 ? 'w-3/4' : 'w-1/2'" />
        </div>
      </div>
    </div>
    <div v-else class="grid gap-4" aria-hidden="true">
      <div v-for="row in variant === 'compact' ? 1 : rows" :key="row" class="flex min-w-0 items-center gap-3">
        <USkeleton class="size-10 shrink-0 rounded-xl" />
        <div class="grid min-w-0 flex-1 gap-2">
          <USkeleton class="h-3 rounded-full" :class="row % 2 ? 'w-2/5' : 'w-1/2'" />
          <USkeleton class="h-3 rounded-full" :class="row % 2 ? 'w-3/4' : 'w-3/5'" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@media (prefers-reduced-motion: reduce) {
  .loading-placeholder :deep(.animate-pulse) { animation: none; }
}
</style>
