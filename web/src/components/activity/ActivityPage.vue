<script setup lang="ts">
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import { useActivity } from '@/composables/useActivity'
import { formatMemberMoney } from '@/utils/displayCurrency'
import BetGamesPanel from './BetGamesPanel.vue'
import DailyCheckInCard from './DailyCheckInCard.vue'
import LuckyDrawPanel from './LuckyDrawPanel.vue'
import DrawExperienceModal from './draw/DrawExperienceModal.vue'
import { useDrawExperience } from './draw/useDrawExperience'
import GroupMessageRewardPanel from './GroupMessageRewardPanel.vue'
import GroupBoostPanel from './GroupBoostPanel.vue'
import ActivityResultDialog from './ActivityResultDialog.vue'

const { overview, result, loading, refreshing, busy, error, load, checkIn, placeBet, draw, clearResult } = useActivity()
const {
  draw: activeDraw, style: drawStyle, phase: drawPhase, result: drawResult,
  error: drawError, resetToken, started: drawStarted, begin: beginDraw, retry: retryDraw,
  markStarted: markDrawStarted,
  reveal: revealDraw, close: closeDraw,
} = useDrawExperience(draw)
</script>

<template>
  <div class="page page--activity">
    <header class="page-header">
      <h1>{{ $t('activity.title') }}</h1>
    </header>
    <template v-if="loading">
      <SkeletonBlock height="9rem" />
      <div class="activity-layout"><SkeletonBlock height="14rem" /><SkeletonBlock height="10rem" /></div>
    </template>
    <template v-else-if="overview">
      <div class="page-toolbar">
        <p>{{ $t('activity.balance', { amount: formatMemberMoney(overview.balance) }) }}</p>
      </div>
      <InlineNotice v-if="error" tone="warning">{{ error }}</InlineNotice>
      <GroupBoostPanel :boost="overview.groupBoost" :refreshing="refreshing" @refresh="load({ quiet: true })" />
      <div class="activity-layout">
        <DailyCheckInCard
          v-if="overview.groupBoost.state === 'boosted'"
          :checked-in="overview.checkedInToday"
          :reward-min-txb-minor="overview.dailyRewardMinTxbMinor"
          :reward-max-txb-minor="overview.dailyRewardMaxTxbMinor"
          :time-zone="overview.timeZone"
          :busy="busy === 'check-in'"
          @check-in="checkIn"
        />
        <GroupMessageRewardPanel v-if="overview.groupBoost.state === 'boosted'" :reward="overview.groupMessageReward" />
        <BetGamesPanel :games="overview.games" :busy="busy === 'bet'" @bet="placeBet" />
        <LuckyDrawPanel
          :draws="overview.draws"
          :busy="Boolean(busy) || Boolean(activeDraw)"
          :drawing-id="activeDraw?.id ?? null"
          :reset-token="resetToken"
          @draw="beginDraw"
        />
      </div>
      <ActivityResultDialog :result="result" @close="clearResult" />
      <DrawExperienceModal
        :draw="activeDraw"
        :style="drawStyle"
        :phase="drawPhase"
        :started="drawStarted"
        :result="drawResult"
        :error="drawError"
        @retry="retryDraw"
        @started="markDrawStarted"
        @reveal="revealDraw"
        @close="closeDraw"
      />
    </template>
    <div v-else class="error-state">
      <h1>{{ $t('errors.activityUnavailable') }}</h1><p>{{ error }}</p>
      <UButton :label="$t('common.tryAgain')" data-haptic="retry" @click="load()" />
    </div>
  </div>
</template>

<style scoped>
.page--activity { display: grid; gap: .65rem; }
.page--activity .page-header, .page--activity .page-toolbar { margin: 0; }
.page--activity .page-header h1 { font-size: 1.4rem; line-height: 1.2; }
.activity-layout { display: grid; gap: .65rem; }
.page--activity :deep(.section-block) { padding: .85rem; }
.page--activity :deep(.section-heading) { margin-bottom: .6rem; }
.page--activity :deep(.empty-inline) { min-height: 0; padding: .65rem; }
.page--activity :deep(.empty-inline h3) { margin: 0; font-size: .9rem; }
.page--activity :deep(.empty-inline p) { margin: .3rem 0 0; font-size: .8rem; }
@media (min-width: 1180px) { .activity-layout { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; } }
</style>
