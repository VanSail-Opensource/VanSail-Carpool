<script setup lang="ts">
import { shallowRef, watch } from 'vue'
import type { SessionStatus } from '@/stores/session'
import { useMotionPreferences } from '@/composables/useMotionPreferences'
import LoadingScreen from './LoadingScreen.vue'
import { useLoadingSequence } from './useLoadingSequence'

const props = withDefaults(defineProps<{ status: SessionStatus; authenticated?: boolean; routeReady?: boolean }>(), { routeReady: true })
const { showing, introducing } = useLoadingSequence(() => props.status, () => props.authenticated === true, () => props.routeReady)
const { reducedMotion } = useMotionPreferences()
const arriving = shallowRef(false)
watch(showing, (active, previous) => {
  arriving.value = previous && !active && props.status === 'ready' && !reducedMotion.value
})
watch(reducedMotion, reduced => { if (reduced) arriving.value = false })
function releaseCover(element: globalThis.Element): void {
  element.setAttribute('inert', '')
  element.setAttribute('aria-hidden', 'true')
}
</script>

<template>
  <div class="session-entrance">
    <div v-if="!showing" class="session-entrance__page" :class="{ 'session-entrance__page--arriving': arriving }">
      <slot />
    </div>
    <Transition name="session-cover" :css="!reducedMotion" @before-leave="releaseCover">
      <div v-if="showing" class="session-entrance__cover">
        <LoadingScreen :animate="introducing" />
      </div>
    </Transition>
  </div>
</template>
