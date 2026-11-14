import type { KnowledgeLesson } from './knowledgeTopics'

export const sceneLabels: Record<KnowledgeLesson, readonly (readonly [string, string])[]> = {
  subscription: [['i-ph-wallet-bold', 'balanceReady'], ['i-ph-clock-bold', 'billingChoice'], ['i-ph-git-branch-bold', 'routingOptional'], ['i-ph-download-simple-bold', 'clientDownload'], ['i-ph-link-bold', 'importLink'], ['i-ph-check-circle-bold', 'nodeSelected']],
  networks: [['i-ph-lightning-bold', 'carriers'], ['i-ph-globe-hemisphere-west-bold', 'globalReach'], ['i-ph-house-line-bold', 'localIsp'], ['i-ph-path-bold', 'exit']],
  payment: [['i-ph-receipt-bold', 'invoice'], ['i-ph-gas-pump-bold', 'gasSeparate'], ['i-ph-bank-bold', 'withdrawal'], ['i-ph-chats-circle-bold', 'bot']],
  sharing: [['i-ph-users-three-bold', 'crowd'], ['i-ph-hard-drives-bold', 'dedicatedIp'], ['i-ph-shield-check-bold', 'controlled'], ['i-ph-fingerprint-bold', 'serviceChecks']],
  rules: [['i-ph-check-circle-bold', 'normalUse'], ['i-ph-user-bold', 'prohibited'], ['i-ph-prohibit-bold', 'blocked'], ['i-ph-gauge-bold', 'connections']],
  why: [['i-ph-clock-bold', 'billingChoice'], ['i-ph-code-bold', 'maintained'], ['i-ph-heartbeat-bold', 'quality'], ['i-ph-chats-circle-bold', 'support']],
}
