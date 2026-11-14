export type KnowledgeLesson = 'networks' | 'payment' | 'sharing' | 'rules' | 'why'
export type KnowledgeTopicId = KnowledgeLesson | 'hourly' | 'traffic' | 'routing'
export interface KnowledgeTopic { id: KnowledgeTopicId; icon: string; points: readonly [string, string, string]; seconds: number }

export const knowledgeTopics: readonly KnowledgeTopic[] = [
  { id: 'networks', icon: 'i-ph-path-bold', points: ['i-ph-lightning-bold', 'i-ph-globe-hemisphere-west-bold', 'i-ph-house-line-bold'], seconds: 26 },
  { id: 'payment', icon: 'i-ph-currency-circle-dollar-bold', points: ['i-ph-git-branch-bold', 'i-ph-equals-bold', 'i-ph-gas-pump-bold'], seconds: 26 },
  { id: 'sharing', icon: 'i-ph-users-three-bold', points: ['i-ph-users-three-bold', 'i-ph-hard-drives-bold', 'i-ph-shield-check-bold'], seconds: 26 },
  { id: 'rules', icon: 'i-ph-shield-check-bold', points: ['i-ph-check-circle-bold', 'i-ph-prohibit-bold', 'i-ph-gauge-bold'], seconds: 26 },
  { id: 'why', icon: 'i-ph-sparkle-bold', points: ['i-ph-clock-bold', 'i-ph-code-bold', 'i-ph-chats-circle-bold'], seconds: 26 },
  { id: 'hourly', icon: 'i-ph-clock-bold', points: ['i-ph-play-circle-bold', 'i-ph-circles-three-bold', 'i-ph-stop-circle-bold'], seconds: 31 },
  { id: 'traffic', icon: 'i-ph-arrows-down-up-bold', points: ['i-ph-plugs-connected-bold', 'i-ph-cube-bold', 'i-ph-hourglass-bold'], seconds: 26 },
  { id: 'routing', icon: 'i-ph-git-branch-bold', points: ['i-ph-sign-in-bold', 'i-ph-funnel-bold', 'i-ph-sign-out-bold'], seconds: 23 },
]

export const chapterKeys = ['first', 'second', 'third', 'fourth'] as const
export const lessonIcons: Record<KnowledgeLesson, readonly string[]> = {
  networks: ['i-ph-lightning-bold', 'i-ph-globe-hemisphere-west-bold', 'i-ph-house-line-bold', 'i-ph-path-bold'],
  payment: ['i-ph-qr-code-bold', 'i-ph-gas-pump-bold', 'i-ph-equals-bold', 'i-ph-chats-circle-bold'],
  sharing: ['i-ph-users-three-bold', 'i-ph-hard-drives-bold', 'i-ph-shield-check-bold', 'i-ph-fingerprint-bold'],
  rules: ['i-ph-check-circle-bold', 'i-ph-robot-bold', 'i-ph-prohibit-bold', 'i-ph-gauge-bold'],
  why: ['i-ph-clock-bold', 'i-ph-code-bold', 'i-ph-heartbeat-bold', 'i-ph-chats-circle-bold'],
}

export function isKnowledgeLesson(id: KnowledgeTopicId): id is KnowledgeLesson {
  return id !== 'hourly' && id !== 'traffic' && id !== 'routing'
}
