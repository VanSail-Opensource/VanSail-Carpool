export type KnowledgeLesson = 'subscription' | 'networks' | 'payment' | 'sharing' | 'rules' | 'why'
export interface KnowledgeChapter {
  key: string
  icon: string
  sceneStep: number
  pointIcons: readonly [string, string, string]
}

const chapter = (key: string, icon: string, sceneStep: number, pointIcons: KnowledgeChapter['pointIcons']): KnowledgeChapter => ({ key, icon, sceneStep, pointIcons })

/** Two short beats retain each continuous visual stage while exposing its details. */
export const lessonChapters: Record<KnowledgeLesson, readonly KnowledgeChapter[]> = {
  subscription: [
    chapter('balance', 'i-ph-wallet-bold', 0, ['i-ph-plus-circle', 'i-ph-receipt', 'i-ph-check-circle']),
    chapter('billing', 'i-ph-currency-circle-dollar-bold', 1, ['i-ph-clock', 'i-ph-arrows-down-up', 'i-ph-wallet']),
    chapter('routing', 'i-ph-git-branch-bold', 2, ['i-ph-sliders-horizontal', 'i-ph-path', 'i-ph-check-circle']),
    chapter('client', 'i-ph-download-simple-bold', 3, ['i-ph-devices', 'i-ph-desktop', 'i-ph-device-mobile']),
    chapter('import', 'i-ph-copy-bold', 4, ['i-ph-link', 'i-ph-clipboard-text', 'i-ph-arrows-clockwise']),
    chapter('connect', 'i-ph-plugs-connected-bold', 5, ['i-ph-hard-drives', 'i-ph-play-circle', 'i-ph-globe']),
  ],
  networks: [
    chapter('entry', 'i-ph-lightning-bold', 0, ['i-ph-lightning', 'i-ph-sign-in', 'i-ph-lock-key']),
    chapter('conditions', 'i-ph-gauge-bold', 0, ['i-ph-cell-signal-full', 'i-ph-map-pin', 'i-ph-traffic-signal']),
    chapter('international', 'i-ph-globe-hemisphere-west-bold', 1, ['i-ph-globe', 'i-ph-path', 'i-ph-sign-out']),
    chapter('reputation', 'i-ph-fingerprint-bold', 1, ['i-ph-buildings', 'i-ph-fingerprint', 'i-ph-chart-line']),
    chapter('broadband', 'i-ph-house-line-bold', 2, ['i-ph-house-line', 'i-ph-chats-circle', 'i-ph-shopping-bag']),
    chapter('services', 'i-ph-shield-check-bold', 2, ['i-ph-identification-card', 'i-ph-user-circle', 'i-ph-sparkle']),
    chapter('pairing', 'i-ph-path-bold', 3, ['i-ph-sign-in', 'i-ph-sign-out', 'i-ph-browser']),
    chapter('destination', 'i-ph-browser-bold', 3, ['i-ph-gauge', 'i-ph-fingerprint', 'i-ph-shield-check']),
  ],
  payment: [
    chapter('invoice', 'i-ph-qr-code-bold', 0, ['i-ph-currency-circle-dollar', 'i-ph-circles-three-plus', 'i-ph-copy']),
    chapter('deadline', 'i-ph-timer-bold', 0, ['i-ph-equals', 'i-ph-timer', 'i-ph-git-branch']),
    chapter('gas', 'i-ph-gas-pump-bold', 1, ['i-ph-receipt', 'i-ph-gas-pump', 'i-ph-wallet']),
    chapter('resources', 'i-ph-coins-bold', 1, ['i-ph-currency-eth', 'i-ph-lightning', 'i-ph-coins']),
    chapter('withdrawal', 'i-ph-bank-bold', 2, ['i-ph-bank', 'i-ph-minus-circle', 'i-ph-equals']),
    chapter('preview', 'i-ph-eye-bold', 2, ['i-ph-eye', 'i-ph-receipt', 'i-ph-warning-circle']),
    chapter('confirmation', 'i-ph-check-circle-bold', 3, ['i-ph-check-circle', 'i-ph-hourglass', 'i-ph-prohibit']),
    chapter('support', 'i-ph-chats-circle-bold', 3, ['i-ph-receipt', 'i-ph-hash', 'i-ph-lock-key']),
  ],
  sharing: [
    chapter('cgnat', 'i-ph-users-three-bold', 0, ['i-ph-house-line', 'i-ph-device-mobile', 'i-ph-globe']),
    chapter('neighbors', 'i-ph-users-bold', 0, ['i-ph-users', 'i-ph-shield-check', 'i-ph-warning-circle']),
    chapter('node', 'i-ph-hard-drives-bold', 1, ['i-ph-hard-drives', 'i-ph-globe', 'i-ph-check-circle']),
    chapter('members', 'i-ph-user-circle-bold', 1, ['i-ph-users-three', 'i-ph-sign-out', 'i-ph-not-equals']),
    chapter('controls', 'i-ph-shield-check-bold', 2, ['i-ph-users', 'i-ph-shield-check', 'i-ph-gauge']),
    chapter('boundaries', 'i-ph-prohibit-bold', 2, ['i-ph-robot', 'i-ph-users-three', 'i-ph-prohibit']),
    chapter('signals', 'i-ph-fingerprint-bold', 3, ['i-ph-fingerprint', 'i-ph-device-mobile', 'i-ph-user-circle']),
    chapter('support', 'i-ph-chats-circle-bold', 3, ['i-ph-shield-check', 'i-ph-warning-circle', 'i-ph-chats-circle']),
  ],
  rules: [
    chapter('everyday', 'i-ph-check-circle-bold', 0, ['i-ph-browser', 'i-ph-play-circle', 'i-ph-game-controller']),
    chapter('platforms', 'i-ph-briefcase-bold', 0, ['i-ph-sparkle', 'i-ph-shopping-bag', 'i-ph-shield-check']),
    chapter('private', 'i-ph-lock-key-bold', 1, ['i-ph-user-circle', 'i-ph-prohibit', 'i-ph-lock-key']),
    chapter('automation', 'i-ph-robot-bold', 1, ['i-ph-robot', 'i-ph-users-three', 'i-ph-warning-circle']),
    chapter('torrents', 'i-ph-prohibit-bold', 2, ['i-ph-arrows-down-up', 'i-ph-envelope', 'i-ph-shield-warning']),
    chapter('abuse', 'i-ph-shield-warning-bold', 2, ['i-ph-bug', 'i-ph-password', 'i-ph-gauge']),
    chapter('controls', 'i-ph-gauge-bold', 3, ['i-ph-shield-check', 'i-ph-gauge', 'i-ph-users-three']),
    chapter('support', 'i-ph-chats-circle-bold', 3, ['i-ph-warning-circle', 'i-ph-prohibit', 'i-ph-chats-circle']),
  ],
  why: [
    chapter('choice', 'i-ph-clock-bold', 0, ['i-ph-clock', 'i-ph-arrows-down-up', 'i-ph-sliders-horizontal']),
    chapter('terms', 'i-ph-receipt-bold', 0, ['i-ph-receipt', 'i-ph-gauge', 'i-ph-play-circle']),
    chapter('project', 'i-ph-code-bold', 1, ['i-ph-code', 'i-ph-user-circle', 'i-ph-hard-drives']),
    chapter('improve', 'i-ph-wrench-bold', 1, ['i-ph-magnifying-glass', 'i-ph-wrench', 'i-ph-code']),
    chapter('quality', 'i-ph-heartbeat-bold', 2, ['i-ph-heartbeat', 'i-ph-magnifying-glass', 'i-ph-user-circle']),
    chapter('recover', 'i-ph-lifebuoy-bold', 2, ['i-ph-warning-circle', 'i-ph-browser', 'i-ph-lifebuoy']),
    chapter('privacy', 'i-ph-lock-key-bold', 3, ['i-ph-eye-slash', 'i-ph-receipt', 'i-ph-chart-bar']),
    chapter('support', 'i-ph-chats-circle-bold', 3, ['i-ph-plugs-connected', 'i-ph-currency-circle-dollar', 'i-ph-chats-circle']),
  ],
}

export const lessonIntervals: Record<KnowledgeLesson, number> = {
  subscription: 3200,
  networks: 2800,
  payment: 2800,
  sharing: 2800,
  rules: 2800,
  why: 2800,
}
