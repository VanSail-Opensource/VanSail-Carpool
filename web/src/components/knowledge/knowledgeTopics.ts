import { lessonChapters, lessonIntervals, type KnowledgeLesson } from './knowledgeLessonChapters'

export { lessonChapters, lessonIntervals } from './knowledgeLessonChapters'
export type { KnowledgeChapter, KnowledgeLesson } from './knowledgeLessonChapters'
export type KnowledgeTopicId = KnowledgeLesson | 'hourly' | 'traffic' | 'routing'
export interface KnowledgeTopic { id: KnowledgeTopicId; icon: string; points: readonly [string, string, string]; seconds: number }

/** Keep displayed runtimes tied to the same chapter counts and clocks as playback. */
export const guideIntervals = { hourly: 3200, traffic: 3200, routing: 3200 } as const
const seconds = (id: KnowledgeLesson) => Math.ceil(lessonChapters[id].length * lessonIntervals[id] / 1000)

export const knowledgeTopics: readonly KnowledgeTopic[] = [
  { id: 'subscription', icon: 'i-ph-rocket-launch-bold', points: ['i-ph-wallet-bold', 'i-ph-download-simple-bold', 'i-ph-plugs-connected-bold'], seconds: seconds('subscription') },
  { id: 'networks', icon: 'i-ph-path-bold', points: ['i-ph-lightning-bold', 'i-ph-globe-hemisphere-west-bold', 'i-ph-house-line-bold'], seconds: seconds('networks') },
  { id: 'payment', icon: 'i-ph-currency-circle-dollar-bold', points: ['i-ph-git-branch-bold', 'i-ph-equals-bold', 'i-ph-gas-pump-bold'], seconds: seconds('payment') },
  { id: 'sharing', icon: 'i-ph-users-three-bold', points: ['i-ph-users-three-bold', 'i-ph-hard-drives-bold', 'i-ph-shield-check-bold'], seconds: seconds('sharing') },
  { id: 'rules', icon: 'i-ph-shield-check-bold', points: ['i-ph-check-circle-bold', 'i-ph-prohibit-bold', 'i-ph-gauge-bold'], seconds: seconds('rules') },
  { id: 'why', icon: 'i-ph-sparkle-bold', points: ['i-ph-clock-bold', 'i-ph-code-bold', 'i-ph-chats-circle-bold'], seconds: seconds('why') },
  { id: 'hourly', icon: 'i-ph-clock-bold', points: ['i-ph-play-circle-bold', 'i-ph-circles-three-bold', 'i-ph-stop-circle-bold'], seconds: Math.ceil(10 * guideIntervals.hourly / 1000) },
  { id: 'traffic', icon: 'i-ph-arrows-down-up-bold', points: ['i-ph-plugs-connected-bold', 'i-ph-cube-bold', 'i-ph-hourglass-bold'], seconds: Math.ceil(9 * guideIntervals.traffic / 1000) },
  { id: 'routing', icon: 'i-ph-git-branch-bold', points: ['i-ph-sign-in-bold', 'i-ph-funnel-bold', 'i-ph-sign-out-bold'], seconds: Math.ceil(8 * guideIntervals.routing / 1000) },
]

export function isKnowledgeLesson(id: KnowledgeTopicId): id is KnowledgeLesson {
  return id !== 'hourly' && id !== 'traffic' && id !== 'routing'
}
