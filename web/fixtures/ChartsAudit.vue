<script setup lang="ts">
import TrafficUsageBar from '@/components/dashboard/TrafficUsageBar.vue'
import StatisticsDonut from '@/components/statistics/StatisticsDonut.vue'
import StatisticsPie from '@/components/statistics/StatisticsPie.vue'
import StatisticsConcentricDonut from '@/components/statistics/StatisticsConcentricDonut.vue'
import StatisticsTrafficChart from '@/components/statistics/StatisticsTrafficChart.vue'
import LineChart from '@/components/charts/LineChart.vue'
import { useI18n } from '@/i18n'
const { t } = useI18n()
const items=[{id:'active',label:'Active',value:3},{id:'queued',label:'Queued',value:1}]
</script>
<template><section class="charts-audit"><TrafficUsageBar :nodes="[{uuid:'Tokyo',name:'Tokyo',countryCode:'JP',totalBytes:'3000'},{uuid:'Frankfurt',name:'Frankfurt',countryCode:'DE',totalBytes:'1000'}]" total-bytes="4000" :catalog-nodes="[]" :use-multiplier="false" /><StatisticsDonut :items="items" :center-label="t('adminUserProfile.entitlements')" center-value="4" :chart-label="t('statistics.subscriptionStates')" /><StatisticsPie :items="items" :chart-label="t('statistics.subscriptionStates')" /><StatisticsConcentricDonut :rings="[{id:'first',label:'First',items},{id:'second',label:'Second',items}]" :center-label="t('adminUserProfile.entitlements')" center-value="4" :chart-label="t('statistics.subscriptionStates')" /><StatisticsTrafficChart :remote="{ weeklyUserIncrease:0,monthlyAverageUsagePercent:0,trafficDates:['2026-10-24','2026-10-25'],trafficSeries:[{uuid:'Tokyo',name:'Tokyo',dailyBytes:['1073741824','2147483648']},{uuid:'Frankfurt',name:'Frankfurt',dailyBytes:['2147483648','1073741824']}] }" /><LineChart :data="[{x:0,label:'First',a:2,b:1},{x:1,label:'Second',a:1,b:3}]" :config="{a:{label:'Tokyo',color:'var(--accent)'},b:{label:'Frankfurt',color:'var(--text-muted)'}}" :label="t('statistics.squadComposition')" bar stacked /></section></template>
<style scoped>.charts-audit { display: grid; gap: 2rem; }</style>
