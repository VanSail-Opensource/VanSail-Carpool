<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import { usePendingStatus } from '@/composables/usePendingStatus'

const props = withDefaults(defineProps<{ animate?: boolean }>(), { animate: true })
const { reducedMotion } = useMotionPreferences()
const { slow } = usePendingStatus(() => true)
const animateOnce = shallowRef(props.animate)
watch(() => reducedMotion.value || !props.animate, stop => { if (stop) animateOnce.value = false }, { immediate: true })
</script>

<template>
  <main class="session-loading" :class="{ 'session-loading--animate': animateOnce }">
    <div class="session-loading__content">
      <h1 class="sr-only">{{ $t('auth.loadingPrefix') }} {{ $t('auth.loadingReplacement') }}</h1>
      <div class="session-loading__brand" aria-hidden="true">
        <span class="session-loading__prefix">
          <span
            v-for="(letter, index) in $t('auth.loadingPrefix')" :key="index"
            class="session-loading__letter" :data-letter="letter"
            :style="{ '--letter-delay': `${Math.min(index * 80, 240)}ms` }"
          >{{ letter }}</span>
        </span>
        <span class="session-loading__words">
          <span class="session-loading__original" lang="zh-CN">
            <span
              v-for="(letter, index) in $t('auth.loadingOriginal')" :key="index"
              class="session-loading__letter" :data-letter="letter"
              :style="{ '--letter-delay': `${Math.min((index + 2) * 80, 240)}ms` }"
            >{{ letter }}</span>
          </span>
          <span class="session-loading__replacement" lang="en">{{ $t('auth.loadingReplacement') }}</span>
        </span>
      </div>
      <p class="session-loading__status" role="status" aria-live="polite" aria-atomic="true">
        {{ $t(slow ? 'auth.loadingAccountSlow' : 'auth.loadingAccount') }}
      </p>
      <div class="startup-placeholder" aria-hidden="true"><USkeleton class="h-3 w-full rounded-full motion-reduce:animate-none" /><USkeleton class="h-3 w-2/3 rounded-full motion-reduce:animate-none" /></div>
    </div>
  </main>
</template>
