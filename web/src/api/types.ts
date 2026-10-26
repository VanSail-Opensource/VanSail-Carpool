import type { components } from './generated'

export type DeepReadonly<T> = T extends (...args: never[]) => unknown
  ? T
  : T extends readonly (infer Item)[]
    ? readonly DeepReadonly<Item>[]
    : T extends object
      ? { readonly [Key in keyof T]: DeepReadonly<T[Key]> }
      : T

// Public request and response contracts come directly from the generated
// OpenAPI components. UI-only view models are defined separately below.
export type ID = components['schemas']['ID']
export type DecimalString = components['schemas']['DecimalInteger']
export type RFC3339 = components['schemas']['Timestamp']
export type Money = DeepReadonly<components['schemas']['Money']>
export type ApiErrorBody = components['schemas']['ApiError']
export type OnboardingStep = components['schemas']['OnboardingStep']
export type User = DeepReadonly<components['schemas']['User']>
export type AuthState = DeepReadonly<components['schemas']['AuthState']>
export type Session = AuthState
export type PMConversation = DeepReadonly<components['schemas']['PMConversation']>
export type PMDelivery = DeepReadonly<components['schemas']['PMDelivery']>
export type ComboControls = DeepReadonly<components['schemas']['ComboControls']>
export type SquadControl = DeepReadonly<components['schemas']['SquadControl']>
export type EarlyActivationQuote = DeepReadonly<components['schemas']['EarlyActivationQuote']>
export type JoinInvite = DeepReadonly<components['schemas']['JoinInvite']>
export type CommunityAccess = DeepReadonly<components['schemas']['CommunityAccess']>
export type CommunityMembership = DeepReadonly<components['schemas']['CommunityMembership']>
export type CommunitySpace = 'group' | 'channel'

export type ResetCadence = components['schemas']['ResetStrategy']
export type SquadProfile = DeepReadonly<components['schemas']['SquadProfile']>
export type SquadProfileWrite = components['schemas']['SquadProfile']
export type BroadbandSquadProfile = components['schemas']['BroadbandSquadProfile']
export type ChinaOptimizedSquadProfile = components['schemas']['ChinaOptimizedSquadProfile']
export type InternationalNetworkSquadProfile = components['schemas']['InternationalNetworkSquadProfile']
export type SquadProduct = DeepReadonly<components['schemas']['SquadProduct']>
export type SquadProductWrite = components['schemas']['SquadProductWrite']
export type Combo = DeepReadonly<components['schemas']['Combo']>
export type CatalogNode = DeepReadonly<components['schemas']['CatalogNode']>
export type Catalog = DeepReadonly<components['schemas']['Catalog']>
export type EntitlementStatus = components['schemas']['EntitlementStatus']
export type Purchase = DeepReadonly<components['schemas']['Purchase']>
export type PurchaseAddonQuote = DeepReadonly<components['schemas']['PurchaseAddonQuote']>
export type AutoRenewal = DeepReadonly<components['schemas']['AutoRenewal']>
export type AutoRenewalFailure = DeepReadonly<components['schemas']['AutoRenewalFailure']>
export interface PurchaseQuote {
  comboId: string
  comboName: string
  grossPrice: Money
  discount: Money
  netPrice: Money
  effectiveAt: RFC3339
  expiresAt: RFC3339
  queued: boolean
  addonSquadUuids: string[]
  accessibleNodes: ProxyNode[]
}
export type ProxyNode = DeepReadonly<components['schemas']['ProxyNode']>
export type TopNode = DeepReadonly<components['schemas']['TopNode']>
export type UsageStatistics = DeepReadonly<components['schemas']['Statistics']>
export type Dashboard = DeepReadonly<components['schemas']['Dashboard']>
export type DashboardNodeUsage = DeepReadonly<components['schemas']['DashboardNodeUsage']>
export type NamedShare = DeepReadonly<components['schemas']['NamedShare']>
export type NormalizedDistribution = DeepReadonly<components['schemas']['NormalizedDistribution']>
export type StatisticsSnapshot = DeepReadonly<components['schemas']['StatisticsSnapshot']>
export type StatisticsNode = DeepReadonly<components['schemas']['StatisticsNode']>
export type StatisticsNodesSnapshot = DeepReadonly<components['schemas']['StatisticsNodesSnapshot']>
export type OperationStatus = components['schemas']['OperationStatus']
export type OperationReceipt = DeepReadonly<components['schemas']['OperationReceipt']>
export type PaymentOperation = DeepReadonly<components['schemas']['PaymentOperation']>
export type ConnectionIP = DeepReadonly<components['schemas']['ConnectionIP']>
export type ConnectionNode = DeepReadonly<components['schemas']['ConnectionNode']>
export type ConnectionScan = DeepReadonly<components['schemas']['ConnectionScan']>
export type IPBlock = DeepReadonly<components['schemas']['IPBlock']>
export type TrafficResetAutomation = DeepReadonly<components['schemas']['TrafficResetAutomation']>
export type DisplayCurrency = components['schemas']['DisplayCurrency']
export type DisplayCurrencyPreference = DeepReadonly<components['schemas']['DisplayCurrencyPreference']>
export type MemberRefundQuote = DeepReadonly<components['schemas']['MemberRefundQuote']>
export type PaymentProvider = components['schemas']['PaymentProvider']
export type PaymentStatus = components['schemas']['PaymentStatus']
export type PaymentMethod = DeepReadonly<components['schemas']['PaymentMethod']>
export type BillingAmountLimits = DeepReadonly<components['schemas']['BillingAmountLimits']>
export type PaymentOrder = DeepReadonly<components['schemas']['PaymentOrder']>
export type AdminPaymentOrder = DeepReadonly<components['schemas']['AdminPaymentOrder']>
export type AdminEntitlement = DeepReadonly<components['schemas']['AdminEntitlement']>
export type Refund = DeepReadonly<components['schemas']['Refund']>
export type LedgerEntry = DeepReadonly<components['schemas']['LedgerEntry']>
export type AdminSetting = DeepReadonly<components['schemas']['AdminSetting']>
export type AdminUserSummary = DeepReadonly<components['schemas']['AdminUserSummary']>
export type BackupRecord = DeepReadonly<components['schemas']['BackupRun']>
export type AuditEvent = DeepReadonly<components['schemas']['AuditEvent']>
export type JobRecord = DeepReadonly<components['schemas']['Job']>
export type HealthState = DeepReadonly<components['schemas']['RuntimeReadiness']>

export interface Paginated<T> {
  items: T[]
  page?: components['schemas']['PageInfo']
}

export type AdminResource =
  | 'pm'
  | 'settings'
  | 'combos'
  | 'squad-products'
  | 'users'
  | 'entitlements'
  | 'payments'
  | 'refunds'
  | 'backups'
  | 'jobs'
  | 'audit-events'
