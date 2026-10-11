<script setup lang="ts">
import InlineNotice from '@/components/common/InlineNotice.vue'
import SkeletonBlock from '@/components/common/SkeletonBlock.vue'
import IPLookupForm from './IPLookupForm.vue'
import IPLookupReport from './IPLookupReport.vue'
import IPLiveDetails from './IPLiveDetails.vue'
import IPLookupProgress from './IPLookupProgress.vue'
import { useIPLookup } from './useIPLookup'
import { ipLookupErrorText } from './presentation'

const { ip, state, quote, refreshQuote, check, displayCheck, report, requestedIP, cacheMatch, loading, busy, errorCode, validIP, submit, load } = useIPLookup()
</script>

<template>
  <main class="page ip-lookup-page">
    <header class="page-header lookup-header">
      <h1 class="lookup-title">{{ $t('ipLookup.title') }}</h1>
    </header>
    <SkeletonBlock v-if="loading" height="12rem" />
    <template v-else>
      <InlineNotice v-if="errorCode" tone="warning">{{ ipLookupErrorText(errorCode) }}</InlineNotice>
      <InlineNotice v-if="state && !state.enabled" tone="info">{{ $t('ipLookup.disabled') }}</InlineNotice>
      <IPLookupForm v-if="state?.enabled" v-model="ip" :state="state" :quote="quote" :busy="busy" :valid-ip="validIP" @check="submit()" />
      <IPLookupProgress v-if="busy" :progress="check?.progress" />
      <IPLookupReport v-if="report" :report="report" :requested-ip="requestedIP" :cache-match="cacheMatch" :check="displayCheck" />
      <p v-else-if="!busy && state?.enabled" class="ip-lookup-empty">{{ $t('ipLookup.empty') }}</p>
      <IPLiveDetails v-if="report" :report="report" :requested-ip="requestedIP" :snapshot="check?.liveDetails ?? null" :refresh-quote="refreshQuote" :busy="busy" @refresh="submit(true)" />
      <UButton v-if="!state" color="neutral" variant="outline" :label="$t('common.tryAgain')" @click="load" />
    </template>
  </main>
</template>

<style scoped>
.ip-lookup-page { max-width: 1080px; }
.lookup-header { margin-bottom: 1rem; }
.lookup-title { font-size: 1.5rem; }
.ip-lookup-empty { padding: 2rem 0; color: var(--text-muted); }
</style>
