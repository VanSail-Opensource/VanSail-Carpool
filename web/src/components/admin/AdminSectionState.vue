<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { useI18n } from '@/i18n'

const props = defineProps<{
  loading: boolean
  error?: string | null
}>()

defineEmits<{ retry: [] }>()
const { t } = useI18n()
const { reducedMotion } = useMotionPreferences()
const hasContent = shallowRef(false)
watch(() => [props.loading, props.error] as const, ([loading, error]) => {
  if (!loading && !error) hasContent.value = true
}, { immediate: true })
</script>

<template>
  <div class="admin-region">
    <p class="sr-only" role="status" aria-live="polite" aria-atomic="true">{{ loading ? t('adminSection.loadingContent') : error ? `${t('adminSection.unavailable')} ${error}` : '' }}</p>
    <div :aria-busy="loading">
      <div v-if="error && !loading" class="error-state error-state--compact">
        <h2>{{ t('adminSection.unavailable') }}</h2>
        <p>{{ error }}</p>
        <UButton color="primary" icon="i-ph-arrow-clockwise" :label="t('adminSection.retry')" :disabled="loading" @click="$emit('retry')" />
      </div>
      <Transition name="admin-state" :css="!reducedMotion">
        <div
          v-if="loading && !hasContent"
          key="loading"
          class="admin-loading"
        >
          <SkeletonBlock height="5rem" />
          <SkeletonBlock height="5rem" />
          <SkeletonBlock height="5rem" />
        </div>
        <div
          v-else-if="hasContent"
          key="content"
        >
          <slot />
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.admin-state-enter-active { transition: opacity 180ms ease-out; }
.admin-state-enter-from { opacity: 0; }
.admin-state-leave-active { display: none; }
</style>
