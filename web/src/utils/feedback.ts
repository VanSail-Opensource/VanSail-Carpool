import { isBrowserExperience } from './experience'

type FeedbackProvider = Pick<typeof import('./telegram'), 'selectionHaptic'>
let provider: FeedbackProvider | undefined

export function configureFeedback(next: FeedbackProvider): void { provider = next }
export function selectionHaptic(): void { if (!isBrowserExperience()) provider?.selectionHaptic() }
