<script setup lang="ts">
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useCommunityMembership } from '@/composables/useCommunityMembership'
import CommunityMembershipRows from './CommunityMembershipRows.vue'

const { membership, loading, refreshing, joining, error, load, join } = useCommunityMembership()
</script>

<template>
  <div class="page page--community">
    <header class="page-header community-page__header">
      <div class="community-page__header-copy">
        <p class="eyebrow">{{ $t('dashboard.aroundTx') }}</p>
        <h1>{{ $t('community.title') }}</h1>
        <p>{{ $t('community.subtitle') }}</p>
      </div>
    </header>

    <InlineNotice tone="info">{{ $t('community.accessNote') }}</InlineNotice>

    <section v-if="loading" class="community-page__loading" aria-live="polite">
      <h2>{{ $t('community.loadingTitle') }}</h2>
      <p>{{ $t('community.loadingDescription') }}</p>
      <SkeletonBlock height="9.5rem" />
    </section>
    <template v-else>
      <InlineNotice v-if="error" tone="warning" :title="$t('community.errorTitle')">{{ error }}</InlineNotice>
      <div class="community-page__workspace">
        <section class="community-page__spaces" aria-labelledby="spaces-title">
          <div class="community-page__spaces-heading">
            <div>
              <p class="community-page__spaces-eyebrow">{{ $t('community.spacesEyebrow') }}</p>
              <h2 id="spaces-title">{{ $t('community.spacesTitle') }}</h2>
            </div>
            <UButton
              color="neutral"
              variant="outline"
              size="lg"
              icon="i-ph-arrows-clockwise"
              class="community-page__refresh"
              :loading="refreshing"
              :label="$t('common.refresh')"
              :aria-label="$t('common.refresh')"
              data-haptic="refresh"
              @click="load()"
            />
          </div>
          <CommunityMembershipRows
            v-if="membership"
            :active-combo="membership.activeCombo"
            :group-joined="membership.groupJoined"
            :channel-joined="membership.channelJoined"
            :joining="joining"
            @join="join"
          />
        </section>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page--community { display: grid; gap: 1rem; }
.community-page__header { display: grid; gap: 0.9rem; margin-bottom: 0.15rem; }
.community-page__header-copy { min-width: 0; }
.community-page__header p:last-child, .community-page__loading p { margin: 0.35rem 0 0; color: var(--text-muted); }
.community-page__loading { display: grid; gap: 0.75rem; }
.community-page__loading h2 { margin: 0; font-size: 1rem; }
.community-page__workspace { display: grid; gap: 1rem; }
.community-page__spaces { min-width: 0; }
.community-page__spaces-heading { display: flex; align-items: end; justify-content: space-between; gap: 0.75rem; margin: 0 0 0.65rem; }
.community-page__spaces-heading h2, .community-page__spaces-eyebrow { margin: 0; }
.community-page__spaces-heading h2 { font-size: 1.08rem; line-height: 1.2; }
.community-page__spaces-eyebrow { margin-bottom: 0.25rem; color: var(--text-faint); font-size: 0.67rem; font-weight: 800; letter-spacing: 0.08em; text-transform: uppercase; }
.community-page__refresh { min-height: 44px; flex: 0 0 auto; }

@media (min-width: 900px) {
  .page--community { max-width: 1060px; }
  .community-page__header { display: flex; align-items: end; justify-content: space-between; gap: 2rem; }
  .community-page__workspace { grid-template-columns: minmax(0, 1fr); }
}

@media (max-width: 420px) {
  .community-page__spaces-heading { align-items: center; }
  .community-page__refresh { width: 44px; padding-inline: 0; }
  .community-page__refresh :deep([data-slot='label']) { display: none; }
}
</style>
