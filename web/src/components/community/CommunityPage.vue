<script setup lang="ts">
import { computed } from 'vue'
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useCommunityMembership } from '@/composables/useCommunityMembership'
import CommunityMembershipRows from './CommunityMembershipRows.vue'

const { membership, loading, refreshing, liveReady, joining, error, load, join } = useCommunityMembership()
const joinedCount = computed(() => Number(membership.value?.groupJoined ?? false) + Number(membership.value?.channelJoined ?? false))
</script>

<template>
  <div class="page page--community" :aria-busy="loading || refreshing">
    <header class="page-header community-page__header">
      <div class="community-page__header-copy">
        <p class="eyebrow">{{ $t('dashboard.aroundTx') }}</p>
        <h1>{{ $t('community.title') }}</h1>
        <p>{{ $t('community.subtitle') }}</p>
      </div>
      <UButton
        color="neutral" variant="outline" icon="i-ph-arrows-clockwise-bold"
        class="community-page__refresh" :loading="refreshing" :disabled="loading || refreshing"
        :label="$t('common.refresh')" :aria-label="$t('common.refresh')"
        data-haptic="refresh" @click="load()"
      />
    </header>

    <section v-if="loading" class="community-page__loading" role="status" aria-live="polite">
      <span>{{ $t('community.loadingTitle') }}</span>
      <div class="community-page__skeletons"><SkeletonBlock height="15rem" /><SkeletonBlock height="15rem" /></div>
    </section>
    <template v-else>
      <InlineNotice v-if="error" tone="warning" :title="$t('community.errorTitle')">{{ error }}</InlineNotice>
      <template v-if="membership">
        <div class="community-page__status">
          <span><UIcon name="i-ph-users-three-bold" aria-hidden="true" />{{ $t('community.joinedCount', { count: joinedCount }) }}</span>
          <span v-if="!refreshing && liveReady" :class="{ 'community-page__ready': membership.activeCombo }"><UIcon :name="membership.activeCombo ? 'i-ph-check-circle-fill' : 'i-ph-lock-simple-bold'" aria-hidden="true" />{{ $t(membership.activeCombo ? 'community.accessReady' : 'community.accessLocked') }}</span>
        </div>
        <CommunityMembershipRows
          :active-combo="membership.activeCombo" :group-joined="membership.groupJoined"
          :channel-joined="membership.channelJoined" :joining="joining"
          :disabled="!liveReady || refreshing" @join="join"
        />
        <footer class="community-page__footer">
          <p><UIcon name="i-ph-telegram-logo-bold" aria-hidden="true" /><span>{{ $t('community.telegramHint') }}</span></p>
          <UButton v-if="!membership.activeCombo && joinedCount < 2" to="/catalog" color="neutral" variant="soft" trailing-icon="i-ph-arrow-up-right-bold" :label="$t('community.exploreAccess')" />
        </footer>
      </template>
      <section v-else class="community-page__empty">
        <UIcon name="i-ph-plugs-connected-bold" aria-hidden="true" />
        <h2>{{ $t('community.errorTitle') }}</h2>
        <p>{{ $t('community.errorDescription') }}</p>
      </section>
    </template>
  </div>
</template>

<style scoped>
.page--community { display: grid; gap: 1.2rem; max-width: 1120px; }
.community-page__header { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin: 0; }
.community-page__header-copy { min-width: 0; }
.community-page__header-copy p:last-child { margin: .45rem 0 0; color: var(--text-muted); }
.community-page__refresh { flex: 0 0 auto; min-height: 44px; border-radius: var(--radius-control); }
.community-page__loading, .community-page__skeletons { display: grid; gap: 1rem; }
.community-page__loading > span { color: var(--text-muted); font-size: .8rem; }
.community-page__status { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .65rem 1rem; font-size: .74rem; color: var(--text-muted); }
.community-page__status > span { display: inline-flex; align-items: center; gap: .45rem; }
.community-page__status .community-page__ready { color: var(--accent); }
.community-page__footer { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.community-page__footer p { display: flex; align-items: center; gap: .6rem; margin: 0; color: var(--text-faint); font-size: .76rem; line-height: 1.6; }
.community-page__footer p > :first-child { flex: 0 0 auto; font-size: 1.2rem; }
.community-page__empty { display: grid; justify-items: center; gap: .75rem; padding: 3rem 1rem; text-align: center; }
.community-page__empty > :first-child { font-size: 2.5rem; color: var(--text-faint); }
.community-page__empty h2, .community-page__empty p { margin: 0; }
.community-page__empty h2 { font-size: 1rem; }
.community-page__empty p { color: var(--text-muted); font-size: .82rem; }
@media (min-width: 700px) { .community-page__skeletons { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 520px) {
  .community-page__header { align-items: start; }
  .community-page__refresh { width: 44px; padding-inline: 0; justify-content: center; }
  .community-page__refresh :deep([data-slot='label']) { display: none; }
  .community-page__footer { align-items: stretch; flex-direction: column; }
  .community-page__footer > :last-child { justify-content: center; }
}
</style>
