import enIPLookup from '../../locales/en/ip-lookup.json'
import enAdminUsdt from '../../locales/en/admin-usdt.json'
import zhAdminUsdt from '../../locales/zh-CN/admin-usdt.json'
import zhIPLookup from '../../locales/zh-CN/ip-lookup.json'
import enIPLookupChecks from '../../locales/en/ip-lookup-checks.json'
import zhIPLookupChecks from '../../locales/zh-CN/ip-lookup-checks.json'
import enIPLookupLive from '../../locales/en/ip-lookup-live.json'
import zhIPLookupLive from '../../locales/zh-CN/ip-lookup-live.json'
import enAdminActivity from '../../locales/en/admin-activity.json'
import enAdminDraw from '../../locales/en/admin-draw.json'
import enAdminCatalog from '../../locales/en/admin-catalog.json'
import enAdminCompensation from '../../locales/en/admin-compensation.json'
import enAdminAbuse from '../../locales/en/admin-abuse.json'
import enAbuse from '../../locales/en/abuse.json'
import enAdminCommunity from '../../locales/en/admin-community.json'
import enAdminCore from '../../locales/en/admin-core.json'
import enAdminData from '../../locales/en/admin-data.json'
import enAdminOnboarding from '../../locales/en/admin-onboarding.json'
import enAdminOperations from '../../locales/en/admin-operations.json'
import enAdminWorkflows from '../../locales/en/admin-workflows.json'
import enActivity from '../../locales/en/activity.json'
import enAffiliates from '../../locales/en/affiliates.json'
import enCommerce from '../../locales/en/commerce.json'
import enCommunity from '../../locales/en/community.json'
import enConnections from '../../locales/en/connections.json'
import enHostConnectivity from '../../locales/en/host-connectivity.json'
import enCore from '../../locales/en/core.json'
import enHome from '../../locales/en/home.json'
import enMember from '../../locales/en/member.json'
import enPaymentCrypto from '../../locales/en/payment-crypto.json'
import enRecovery from '../../locales/en/recovery.json'
import enSquadProfile from '../../locales/en/squad-profile.json'
import enStatistics from '../../locales/en/statistics.json'
import enSettings from '../../locales/en/settings.json'
import zhAdminActivity from '../../locales/zh-CN/admin-activity.json'
import zhAdminDraw from '../../locales/zh-CN/admin-draw.json'
import zhAdminCatalog from '../../locales/zh-CN/admin-catalog.json'
import zhAdminCompensation from '../../locales/zh-CN/admin-compensation.json'
import zhAdminAbuse from '../../locales/zh-CN/admin-abuse.json'
import zhAbuse from '../../locales/zh-CN/abuse.json'
import zhAdminCommunity from '../../locales/zh-CN/admin-community.json'
import zhAdminCore from '../../locales/zh-CN/admin-core.json'
import zhAdminData from '../../locales/zh-CN/admin-data.json'
import zhAdminOnboarding from '../../locales/zh-CN/admin-onboarding.json'
import zhAdminOperations from '../../locales/zh-CN/admin-operations.json'
import zhAdminWorkflows from '../../locales/zh-CN/admin-workflows.json'
import zhActivity from '../../locales/zh-CN/activity.json'
import zhAffiliates from '../../locales/zh-CN/affiliates.json'
import zhCommerce from '../../locales/zh-CN/commerce.json'
import zhCommunity from '../../locales/zh-CN/community.json'
import zhConnections from '../../locales/zh-CN/connections.json'
import zhHostConnectivity from '../../locales/zh-CN/host-connectivity.json'
import zhCore from '../../locales/zh-CN/core.json'
import zhHome from '../../locales/zh-CN/home.json'
import zhMember from '../../locales/zh-CN/member.json'
import zhPaymentCrypto from '../../locales/zh-CN/payment-crypto.json'
import zhRecovery from '../../locales/zh-CN/recovery.json'
import zhSquadProfile from '../../locales/zh-CN/squad-profile.json'
import zhStatistics from '../../locales/zh-CN/statistics.json'
import zhSettings from '../../locales/zh-CN/settings.json'

const en = {
 ...enAdminUsdt,
 ipLookup: { ...enIPLookup.ipLookup, ...enIPLookupChecks.ipLookup, refusals: { ...enIPLookup.ipLookup.refusals, ...enIPLookupChecks.ipLookup.refusals } },
 ...enIPLookupLive,
  ...enHostConnectivity,
  ...enSettings,
  ...enCore, ...enRecovery, ...enAffiliates, ...enActivity, ...enCommerce, ...enPaymentCrypto, ...enCommunity, ...enMember, ...enConnections, ...enHome, ...enAdminCore, ...enAdminOnboarding, ...enAdminData,
  ...enAdminActivity, ...enAdminDraw, ...enAdminCommunity, ...enAdminCatalog, ...enAdminOperations, ...enAdminWorkflows, ...enAdminCompensation, ...enAdminAbuse, ...enAbuse, ...enSquadProfile, ...enStatistics,
}
const zhCN: typeof en = {
 ...zhAdminUsdt,
 ipLookup: { ...zhIPLookup.ipLookup, ...zhIPLookupChecks.ipLookup, refusals: { ...zhIPLookup.ipLookup.refusals, ...zhIPLookupChecks.ipLookup.refusals } },
 ...zhIPLookupLive,
  ...zhHostConnectivity,
  ...zhSettings,
  ...zhCore, ...zhRecovery, ...zhAffiliates, ...zhActivity, ...zhCommerce, ...zhPaymentCrypto, ...zhCommunity, ...zhMember, ...zhConnections, ...zhHome, ...zhAdminCore, ...zhAdminOnboarding, ...zhAdminData,
  ...zhAdminActivity, ...zhAdminDraw, ...zhAdminCommunity, ...zhAdminCatalog, ...zhAdminOperations, ...zhAdminWorkflows, ...zhAdminCompensation, ...zhAdminAbuse, ...zhAbuse, ...zhSquadProfile, ...zhStatistics,
}

export const localeMessages = {
  en,
  'zh-CN': zhCN,
} as const

export type Locale = keyof typeof localeMessages
export type LocaleMessages = typeof en

type LeafKeys<Value, Prefix extends string = ''> = Value extends object
  ? { [Key in keyof Value & string]: LeafKeys<Value[Key], Prefix extends '' ? Key : `${Prefix}.${Key}`> }[keyof Value & string]
  : Prefix

export type LocaleKey = LeafKeys<LocaleMessages>
