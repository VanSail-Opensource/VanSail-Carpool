
export interface DatabaseColumn {
  name: string
  declaredType: string
  nullable: boolean
  primaryKeyPosition: number
  insertable?: boolean
  hasDefault?: boolean
  protectionReason?: string
  editable: boolean
  sensitive: boolean
}

export interface DatabaseTable {
  name: string
  columns: DatabaseColumn[]
  allowInsert?: boolean
  allowUpdate?: boolean
  allowDelete?: boolean
  protectionReason?: string
  highRisk: boolean
  warning: string
}

export type DatabaseValue = string | boolean | null | { blobBase64: string }

export interface DatabaseRow {
  key: Record<string, string>
  values: Record<string, DatabaseValue>
  recordHash: string
}

export interface DatabaseRowsPage {
  items: DatabaseRow[]
  nextCursor: string | null
}

export type DatabaseFilterOperator = 'eq' | 'ne' | 'contains' | 'starts_with' | 'gt' | 'gte' | 'lt' | 'lte' | 'is_null' | 'not_null'

export interface DatabaseFilter {
  column: string
  operator: DatabaseFilterOperator
  value?: DatabaseValue
}

export interface DatabaseQueryInput {
  search?: string
  filters: DatabaseFilter[]
  cursor?: string
  limit: number
}

export interface StatisticPoint {
  periodStart: string
  count: number
  uniqueUsers: number
  inputTxbMinor: string
  outputTxbMinor: string
  netTxbMinor: string
}

export interface StatisticSlice {
  id: string
  label: string
  count: number
}

export interface AdminStatistics {
  resourceId: string
  timeZone: string
  from: string
  to: string
  bucket: 'daily' | 'weekly'
  count: number
  uniqueUsers: number
  inputTxbMinor: string
  outputTxbMinor: string
  netTxbMinor: string
  discountTxbMinor: string
  addonTxbMinor: string
  wins: number
  losses: number
  series: StatisticPoint[]
  distribution: StatisticSlice[]
}

export interface StatisticsQuery {
  from?: string
  to?: string
  bucket?: 'daily' | 'weekly'
  timeZone?: string
}

export interface OnboardingWelcomeMessage {
  id: string
  text: string
  durationMs?: number
}

export interface OnboardingAgreement {
  id: string
  icon: 'link-break' | 'shield-check' | 'users-three' | 'warning' | 'lock-key' | 'heart' | 'scales'
  color?: 'accent' | 'success' | 'warning' | 'danger' | 'neutral'
  pageTitle?: string
  title: string
  body: string
}

export type OnboardingLocalizedContent = Record<'en' | 'zh-CN', OnboardingWelcomeMessage[] | OnboardingAgreement[]>

export interface PublishedOnboarding {
  locale: 'en' | 'zh-CN'
  welcomeRevision: number
  agreementRevision: number
  welcome: OnboardingWelcomeMessage[]
  agreements: OnboardingAgreement[]
}

export interface OnboardingBundle {
  kind: 'welcome' | 'agreements'
  draft: OnboardingLocalizedContent
  published: OnboardingLocalizedContent
  draftRevision: number
  publishedRevision: number
}

export interface DatabaseMutationInput {
  action: 'insert' | 'update' | 'delete'
  table: string
  key?: Record<string, string>
  values?: Record<string, DatabaseValue>
  recordHash?: string
  reason: string
}

export interface DatabaseMutationReview {
  action: DatabaseMutationInput['action']
  table: string
  key?: Record<string, string>
  before?: Record<string, DatabaseValue> | null
  after?: Record<string, DatabaseValue> | null
  changedColumns: string[]
  reviewHash: string
  requiredConfirmation: string
  rescueBackupRequired: true
  warning: string
}

export interface DatabaseMutationResult {
  row?: DatabaseRow
  deleted: boolean
  rescueBackupId: string
}
