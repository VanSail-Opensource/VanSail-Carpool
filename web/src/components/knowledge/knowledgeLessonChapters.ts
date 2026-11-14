export type KnowledgeLesson = 'subscription' | 'networks' | 'payment' | 'sharing' | 'rules' | 'why'
export interface KnowledgeChapter {
  key: string
  icon: string
  sceneStep: number
  pointIcons: readonly [string, string, string]
}

const chapter = (key: string, icon: string, sceneStep: number, pointIcons: KnowledgeChapter['pointIcons']): KnowledgeChapter => ({ key, icon, sceneStep, pointIcons })

/** Each short beat has one clear action and a deterministic interface/diagram state. */
export const lessonChapters: Record<KnowledgeLesson, readonly KnowledgeChapter[]> = {
  subscription: [
    chapter('balance', 'i-ph-wallet-bold', 0, ['i-ph-plus-circle', 'i-ph-receipt', 'i-ph-currency-circle-dollar']),
    chapter('credit', 'i-ph-check-circle-bold', 1, ['i-ph-hourglass', 'i-ph-wallet', 'i-ph-check-circle']),
    chapter('billing', 'i-ph-currency-circle-dollar-bold', 2, ['i-ph-clock', 'i-ph-arrows-down-up', 'i-ph-sliders-horizontal']),
    chapter('access', 'i-ph-plugs-connected-bold', 3, ['i-ph-play-circle', 'i-ph-arrows-down-up', 'i-ph-wallet']),
    chapter('routing', 'i-ph-git-branch-bold', 4, ['i-ph-sliders-horizontal', 'i-ph-path', 'i-ph-arrow-bend-up-right']),
    chapter('applied', 'i-ph-check-circle-bold', 5, ['i-ph-floppy-disk', 'i-ph-hourglass', 'i-ph-check-circle']),
    chapter('client', 'i-ph-download-simple-bold', 6, ['i-ph-devices', 'i-ph-desktop', 'i-ph-device-mobile']),
    chapter('compatibility', 'i-ph-shield-check-bold', 7, ['i-ph-devices', 'i-ph-check-circle', 'i-ph-hard-drives']),
    chapter('import', 'i-ph-copy-bold', 8, ['i-ph-link', 'i-ph-clipboard-text', 'i-ph-lock-key']),
    chapter('refresh', 'i-ph-arrows-clockwise-bold', 9, ['i-ph-arrows-clockwise', 'i-ph-list-checks', 'i-ph-hourglass']),
    chapter('connect', 'i-ph-plugs-connected-bold', 10, ['i-ph-hard-drives', 'i-ph-power', 'i-ph-globe']),
    chapter('verify', 'i-ph-browser-bold', 11, ['i-ph-browser', 'i-ph-arrows-clockwise', 'i-ph-lock-key']),
  ],
  networks: [
    chapter('entry', 'i-ph-lightning-bold', 0, ['i-ph-lightning', 'i-ph-sign-in', 'i-ph-lock-key']),
    chapter('conditions', 'i-ph-gauge-bold', 1, ['i-ph-cell-signal-full', 'i-ph-map-pin', 'i-ph-traffic-signal']),
    chapter('international', 'i-ph-globe-hemisphere-west-bold', 2, ['i-ph-globe', 'i-ph-path', 'i-ph-sign-out']),
    chapter('reputation', 'i-ph-fingerprint-bold', 3, ['i-ph-buildings', 'i-ph-fingerprint', 'i-ph-chart-line']),
    chapter('broadband', 'i-ph-house-line-bold', 4, ['i-ph-house-line', 'i-ph-chats-circle', 'i-ph-shopping-bag']),
    chapter('services', 'i-ph-shield-check-bold', 5, ['i-ph-identification-card', 'i-ph-user-circle', 'i-ph-sparkle']),
    chapter('pairing', 'i-ph-path-bold', 6, ['i-ph-sign-in', 'i-ph-funnel', 'i-ph-sign-out']),
    chapter('destination', 'i-ph-browser-bold', 7, ['i-ph-gauge', 'i-ph-fingerprint', 'i-ph-browser']),
    chapter('compare', 'i-ph-chart-line-bold', 8, ['i-ph-timer', 'i-ph-waveform', 'i-ph-download-simple']),
    chapter('choose', 'i-ph-check-circle-bold', 9, ['i-ph-lightning', 'i-ph-house-line', 'i-ph-arrow-right']),
  ],
  payment: [
    chapter('invoice', 'i-ph-receipt-bold', 0, ['i-ph-currency-circle-dollar', 'i-ph-circles-three-plus', 'i-ph-copy']),
    chapter('network', 'i-ph-git-branch-bold', 1, ['i-ph-git-branch', 'i-ph-qr-code', 'i-ph-check-circle']),
    chapter('deadline', 'i-ph-timer-bold', 2, ['i-ph-equals', 'i-ph-timer', 'i-ph-receipt']),
    chapter('provider', 'i-ph-bank-bold', 3, ['i-ph-bank', 'i-ph-sliders-horizontal', 'i-ph-minus-circle']),
    chapter('withdrawal', 'i-ph-plus-circle-bold', 4, ['i-ph-receipt', 'i-ph-plus-circle', 'i-ph-bank']),
    chapter('preview', 'i-ph-eye-bold', 5, ['i-ph-minus-circle', 'i-ph-equals', 'i-ph-check-circle']),
    chapter('gas', 'i-ph-gas-pump-bold', 6, ['i-ph-equals', 'i-ph-gas-pump', 'i-ph-wallet']),
    chapter('resources', 'i-ph-coins-bold', 7, ['i-ph-currency-eth', 'i-ph-lightning', 'i-ph-coins']),
    chapter('confirmation', 'i-ph-hourglass-bold', 8, ['i-ph-paper-plane-tilt', 'i-ph-hourglass', 'i-ph-check-circle']),
    chapter('credited', 'i-ph-wallet-bold', 9, ['i-ph-wallet', 'i-ph-receipt', 'i-ph-prohibit']),
    chapter('support', 'i-ph-chats-circle-bold', 10, ['i-ph-receipt', 'i-ph-git-branch', 'i-ph-hash']),
    chapter('secrets', 'i-ph-lock-key-bold', 11, ['i-ph-key', 'i-ph-password', 'i-ph-eye-slash']),
  ],
  sharing: [
    chapter('cgnat', 'i-ph-users-three-bold', 0, ['i-ph-house-line', 'i-ph-device-mobile', 'i-ph-globe']),
    chapter('neighbors', 'i-ph-users-bold', 1, ['i-ph-users', 'i-ph-shield-check', 'i-ph-warning-circle']),
    chapter('node', 'i-ph-hard-drives-bold', 2, ['i-ph-hard-drives', 'i-ph-globe', 'i-ph-check-circle']),
    chapter('members', 'i-ph-user-circle-bold', 3, ['i-ph-users-three', 'i-ph-sign-out', 'i-ph-not-equals']),
    chapter('controls', 'i-ph-shield-check-bold', 4, ['i-ph-users', 'i-ph-shield-check', 'i-ph-gauge']),
    chapter('boundaries', 'i-ph-prohibit-bold', 5, ['i-ph-robot', 'i-ph-users-three', 'i-ph-prohibit']),
    chapter('signals', 'i-ph-fingerprint-bold', 6, ['i-ph-fingerprint', 'i-ph-device-mobile', 'i-ph-user-circle']),
    chapter('behavior', 'i-ph-chart-line-bold', 7, ['i-ph-user-circle', 'i-ph-shield-check', 'i-ph-prohibit']),
    chapter('limits', 'i-ph-warning-circle-bold', 8, ['i-ph-shield-check', 'i-ph-identification-card', 'i-ph-warning-circle']),
    chapter('support', 'i-ph-chats-circle-bold', 9, ['i-ph-hard-drives', 'i-ph-clock', 'i-ph-chats-circle']),
  ],
  rules: [
    chapter('everyday', 'i-ph-check-circle-bold', 0, ['i-ph-browser', 'i-ph-play-circle', 'i-ph-game-controller']),
    chapter('platforms', 'i-ph-briefcase-bold', 1, ['i-ph-sparkle', 'i-ph-shopping-bag', 'i-ph-shield-check']),
    chapter('private', 'i-ph-lock-key-bold', 2, ['i-ph-user-circle', 'i-ph-prohibit', 'i-ph-lock-key']),
    chapter('subscription', 'i-ph-link-bold', 3, ['i-ph-link', 'i-ph-eye-slash', 'i-ph-lock-key']),
    chapter('automation', 'i-ph-robot-bold', 4, ['i-ph-robot', 'i-ph-users-three', 'i-ph-warning-circle']),
    chapter('torrents', 'i-ph-prohibit-bold', 5, ['i-ph-arrows-down-up', 'i-ph-envelope', 'i-ph-shield-warning']),
    chapter('abuse', 'i-ph-shield-warning-bold', 6, ['i-ph-bug', 'i-ph-password', 'i-ph-gauge']),
    chapter('controls', 'i-ph-gauge-bold', 7, ['i-ph-shield-check', 'i-ph-gauge', 'i-ph-users-three']),
    chapter('limits', 'i-ph-warning-circle-bold', 8, ['i-ph-warning-circle', 'i-ph-prohibit', 'i-ph-shield-check']),
    chapter('support', 'i-ph-chats-circle-bold', 9, ['i-ph-browser', 'i-ph-clock', 'i-ph-chats-circle']),
  ],
  why: [
    chapter('choice', 'i-ph-clock-bold', 0, ['i-ph-clock', 'i-ph-arrows-down-up', 'i-ph-sliders-horizontal']),
    chapter('terms', 'i-ph-receipt-bold', 1, ['i-ph-receipt', 'i-ph-gauge', 'i-ph-play-circle']),
    chapter('project', 'i-ph-code-bold', 2, ['i-ph-code', 'i-ph-user-circle', 'i-ph-hard-drives']),
    chapter('improve', 'i-ph-wrench-bold', 3, ['i-ph-magnifying-glass', 'i-ph-wrench', 'i-ph-code']),
    chapter('quality', 'i-ph-heartbeat-bold', 4, ['i-ph-heartbeat', 'i-ph-magnifying-glass', 'i-ph-user-circle']),
    chapter('diagnose', 'i-ph-magnifying-glass-bold', 5, ['i-ph-hard-drives', 'i-ph-clock', 'i-ph-browser']),
    chapter('recover', 'i-ph-lifebuoy-bold', 6, ['i-ph-warning-circle', 'i-ph-browser', 'i-ph-lifebuoy']),
    chapter('privacy', 'i-ph-eye-slash-bold', 7, ['i-ph-eye-slash', 'i-ph-plugs-connected', 'i-ph-clock']),
    chapter('records', 'i-ph-receipt-bold', 8, ['i-ph-user-circle', 'i-ph-receipt', 'i-ph-chart-bar']),
    chapter('support', 'i-ph-chats-circle-bold', 9, ['i-ph-plugs-connected', 'i-ph-currency-circle-dollar', 'i-ph-chats-circle']),
  ],
}

export const lessonIntervals: Record<KnowledgeLesson, number> = {
  subscription: 3200,
  networks: 3200,
  payment: 3200,
  sharing: 3200,
  rules: 3200,
  why: 3200,
}
