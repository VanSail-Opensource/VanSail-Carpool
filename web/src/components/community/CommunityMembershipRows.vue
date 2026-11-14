<script setup lang="ts">
import { motion } from 'motion-v'
import type { CommunitySpace } from '@/api/types'
import { useMotionPreferences } from '@/composables/useMotionPreferences'

const props = defineProps<{
  activeCombo: boolean
  groupJoined: boolean
  channelJoined: boolean
  joining: readonly CommunitySpace[]
  disabled?: boolean
}>()
const emit = defineEmits<{ join: [space: CommunitySpace] }>()
const { reducedMotion } = useMotionPreferences()
const spaces: ReadonlyArray<{ kind: CommunitySpace; icon: string; topicIcons: readonly string[] }> = [
  { kind: 'group', icon: 'i-ph-chats-circle-bold', topicIcons: ['i-ph-chat-circle-dots-bold', 'i-ph-lightbulb-bold'] },
  { kind: 'channel', icon: 'i-ph-broadcast-bold', topicIcons: ['i-ph-megaphone-bold', 'i-ph-git-branch-bold'] },
]
function state(space: CommunitySpace): 'joined' | 'unavailable' | 'ready' {
  if (space === 'group' ? props.groupJoined : props.channelJoined) return 'joined'
  return props.activeCombo ? 'ready' : 'unavailable'
}
function join(space: CommunitySpace): void {
  if (!props.disabled && state(space) === 'ready' && !props.joining.includes(space)) emit('join', space)
}
</script>

<template>
  <section class="community-rows" :aria-label="$t('community.title')">
    <motion.article
      v-for="(space, index) in spaces" :key="space.kind" class="community-row"
      :class="`community-row--${state(space.kind)}`"
      :initial="reducedMotion ? false : { opacity: 0, y: 8 }"
      :animate="{ opacity: 1, y: 0 }" :transition="{ duration: .28, delay: reducedMotion ? 0 : index * .06 }"
    >
      <header class="community-row__top">
        <span class="community-row__icon" aria-hidden="true"><UIcon :name="space.icon" /></span>
        <UBadge :color="state(space.kind) === 'joined' ? 'success' : 'neutral'" variant="subtle" :icon="state(space.kind) === 'joined' ? 'i-ph-check-circle-fill' : undefined">{{ $t(`community.${state(space.kind)}`) }}</UBadge>
      </header>
      <div class="community-row__copy">
        <h2>{{ $t(`community.${space.kind}Title`) }}</h2>
        <p>{{ $t(`community.${space.kind}Description`) }}</p>
      </div>
      <ul class="community-row__topics">
        <li v-for="(icon, topic) in space.topicIcons" :key="icon"><UIcon :name="icon" aria-hidden="true" />{{ $t(`community.topics.${space.kind}${topic}`) }}</li>
      </ul>
      <footer class="community-row__action">
        <UButton
          v-if="state(space.kind) === 'ready'" color="neutral" variant="solid"
          trailing-icon="i-ph-arrow-up-right-bold" class="community-row__join"
          :loading="joining.includes(space.kind)" :disabled="disabled || joining.includes(space.kind)"
          :label="joining.includes(space.kind) ? $t('community.joining') : $t('community.join')"
          data-haptic="open" @click="join(space.kind)"
        />
        <span v-else class="community-row__state-icon" aria-hidden="true"><UIcon :name="state(space.kind) === 'joined' ? 'i-ph-check-bold' : 'i-ph-lock-simple-bold'" /></span>
        <p>{{ $t(`community.${state(space.kind)}Description`) }}</p>
      </footer>
    </motion.article>
  </section>
</template>

<style scoped>
.community-rows { display: grid; gap: .9rem; }
.community-row { display: flex; flex-direction: column; gap: 1.1rem; min-width: 0; padding: 1.2rem; border: 1px solid var(--line); border-radius: var(--radius-panel); background: var(--surface-raised); transition: border-color .2s; }
.community-row:hover, .community-row:focus-within { border-color: var(--accent-line); }
.community-row__top { display: flex; align-items: center; justify-content: space-between; gap: .8rem; }
.community-row__icon { display: grid; width: 3rem; height: 3rem; flex: 0 0 auto; place-items: center; border: 1px solid var(--accent-line); border-radius: 12px; color: var(--accent); background: var(--accent-soft); font-size: 1.5rem; }
.community-row__copy { min-width: 0; }
.community-row__copy h2, .community-row__copy p, .community-row__action p { margin: 0; }
.community-row__copy h2 { font-size: 1.2rem; letter-spacing: -.025em; }
.community-row__copy p { margin-top: .35rem; color: var(--text-muted); font-size: .82rem; line-height: 1.5; }
.community-row__topics { display: flex; flex-wrap: wrap; gap: .6rem 1rem; list-style: none; margin: 0; padding: 0; color: var(--text-muted); font-size: .73rem; }
.community-row__topics li { display: inline-flex; align-items: center; gap: .35rem; }
.community-row__topics li > :first-child { color: var(--accent); }
.community-row__action { display: flex; align-items: center; gap: .85rem; margin-top: auto; padding-top: 1rem; border-top: 1px solid var(--line); }
.community-row__action p { color: var(--text-faint); font-size: .71rem; line-height: 1.5; }
.community-row__join { flex: 0 0 auto; min-width: 6rem; min-height: 44px; justify-content: center; }
.community-row__state-icon { display: grid; flex: 0 0 auto; place-items: center; width: 2rem; height: 2rem; border-radius: 50%; background: var(--surface); color: var(--text-muted); }
.community-row--joined .community-row__state-icon { color: var(--accent); background: var(--accent-soft); }
@media (min-width: 700px) { .community-rows { grid-template-columns: repeat(2, minmax(0, 1fr)); } .community-row { padding: 1.5rem; } }
@media (prefers-reduced-motion: reduce) { .community-row { transition: none; } }
</style>
