import { allBrowserScopes, memberBrowserScopes, adminBrowserScopes, browserRecoverySuites } from './browser-recovery-scopes.mjs'

// Specific ownership rules precede broad fallbacks. Unknown runtime files run all
// scopes, so a new module cannot silently lose browser coverage.
const rules = [
  [/^web\/scripts\/browser-hourly-preferences\.mjs$/, ['hourly']],
  [/^web\/src\/components\/common\/(?:Demo|illustrationTimeline)/, ['demonstrations']],
  [/^web\/src\/components\/catalog\/(?:BillingDemo|HourlyBillingScene|TrafficBillingScene|billingDemo)/, ['demonstrations']],
  [/^web\/src\/components\/diy\/RoutingDemo/, ['demonstrations']],
  [/^web\/src\/components\/admin\/backups\//, ['backups']],
  [/^web\/src\/components\/admin\/database\//, ['admin-database']],
  [/^web\/src\/components\/admin\/activity\//, ['admin-activity']],
  [/^web\/src\/components\/admin\/xnode\//, ['xnode-availability', 'xnode-creation']],
  [/^web\/src\/components\/admin\/(?:usdt\/|Admin(?:Settings|Emby|Payment|Billing|Onboarding))/, ['admin-settings']],
  [/^web\/src\/components\/admin\/(?:AdminBackups|useAdminBackups)/, ['backups']],
  [/^web\/src\/components\/admin\/(?:AdminCoupons|useAdminCoupons)/, ['admin-coupons']],
  [/^web\/src\/components\/admin\/users\/(?:AdminUserPM|useAdminUserPM)/, ['pm']],
  [/^web\/src\/components\/admin\//, adminBrowserScopes],
  [/^web\/src\/components\/(?:hourly|settings)\//, ['hourly', 'settings']],
  [/^web\/src\/components\/(?:catalog|squad-addition|squad-profile)\//, ['checkout', 'purchase', 'hourly', 'demonstrations']],
  [/^web\/src\/components\/diy\//, ['member', 'demonstrations']],
  [/^web\/src\/components\/(?:subscription|connections|community|onboarding|session)\//, ['member']],
  [/^web\/src\/components\/billing\//, ['checkout', 'receipt', 'purchase']],
  [/^web\/src\/components\/activity\//, ['activity']],
  [/^web\/src\/components\/ip-lookup\//, ['ip']],
  [/^web\/src\/components\/(?:charts|statistics)\//, ['charts', 'ip']],
  [/^web\/src\/components\/(?:layout|dashboard|affiliates|emby)\//, memberBrowserScopes],
  [/^web\/src\/composables\/use(?:AdminDatabase|AdminDatabaseMutation)/, ['admin-database']],
  [/^web\/src\/composables\/use(?:Admin|PendingStatus|DurableCommand)/, adminBrowserScopes],
  [/^web\/src\/composables\/use(?:DisplayCurrency)/, ['currency', 'checkout', 'purchase', 'hourly']],
  [/^web\/src\/composables\/(?:use(?:Payment|AutoRenewal|Purchase|PricingQuote|CouponRedemption|SquadAddition)|paymentOrder)/, ['checkout', 'purchase', 'receipt']],
  [/^web\/src\/composables\/(?:useOperationReceipt|operationReceipt)/, ['operation', 'purchase', 'hourly', 'settings']],
  [/^web\/src\/composables\/useActivity/, ['activity']],
  [/^web\/src\/composables\/use(?:Community|Onboarding|IntroSequence)/, ['member']],
  [/^web\/src\/composables\/use(?:IllustrationPlayback|Demo)/, ['demonstrations']],
  [/^web\/src\/composables\/use(?:Telegram|Affiliate|Emby|Dashboard)/, memberBrowserScopes],
  [/^web\/src\/api\/(?:contracts\/)?hourly/, ['hourly', 'settings']],
  [/^web\/src\/api\/(?:preferences|comboControls|memberOperations)/, ['settings', 'purchase', 'operation']],
  [/^web\/src\/api\/(?:ipLookup|contracts\/ipLookup)/, ['ip']],
  [/^web\/src\/api\/(?:xnodeMember|memberConnections)/, ['member']],
  [/^web\/src\/api\/xnode/, ['xnode-availability', 'xnode-creation']],
  [/^web\/src\/api\/(?:payg|purchaseAddons|pricingReads)/, ['purchase', 'checkout']],
  [/^web\/src\/api\/(?:community|contracts\/community)/, ['member']],
  [/^web\/src\/api\/displayCurrency/, ['currency', 'checkout', 'purchase', 'hourly']],
  [/^web\/public\/fonts\/SpaceGrotesk[^/]*\.(?:woff2?|ttf|otf)$/, memberBrowserScopes],
  [/^web\/src\/styles\/(?:fonts|loading)\.css$/, allBrowserScopes],
  [/^web\/src\/styles\/statistics-/, ['charts', 'ip']],
  [/^web\/src\/styles\/billing-0[23]\.css$/, ['checkout', 'receipt', 'admin-settings']],
  [/^web\/src\/styles\//, memberBrowserScopes],
  [/^web\/src\/browser\//, adminBrowserScopes],
  [/^web\/src\/views\/Admin/, adminBrowserScopes],
  [/^web\/src\/(?:views\/|router\/|main\.ts|App\.vue)/, memberBrowserScopes],
  [/^web\/locales\/[^/]+\/(?:hourly|settings)\.json$/, ['hourly', 'settings']],
  [/^web\/locales\/[^/]+\/(?:payg|commerce|squad-profile)\.json$/, ['checkout', 'purchase', 'hourly', 'demonstrations']],
  [/^web\/locales\/[^/]+\/diy(?:-fields)?\.json$/, ['member', 'demonstrations']],
  [/^web\/locales\/[^/]+\/(?:subscription|member(?:-nodes)?|community)\.json$/, ['member']],
  [/^web\/locales\/[^/]+\/ip-lookup/, ['ip']],
  [/^web\/locales\/[^/]+\/(?:charts|statistics|predictions|native-metrics)\.json$/, ['charts', 'ip']],
  [/^web\/locales\/[^/]+\/(?:node-|xnode|firewall|admin-endpoints|managed-routing)/, ['xnode-availability', 'xnode-creation', 'member']],
  [/^web\/locales\/[^/]+\/(?:admin-|browser|api-tokens)/, adminBrowserScopes],
  [/^web\/fixtures\/(?:member-utility|memberUtility|MemberUtility)/, ['member-design']],
  [/^web\/fixtures\/(?:hourly|Hourly)/, ['hourly', 'demonstrations']],
  [/^web\/fixtures\/(?:settings|Settings)/, ['settings', 'hourly']],
  [/^web\/fixtures\/(?:browser-auth|BrowserAuth)/, ['auth', 'settings', 'operation']],
  [/^web\/fixtures\/(?:recovery-audit|RecoveryAudit|recovery-data)/, ['currency', 'checkout', 'member']],
  [/^web\/fixtures\/(?:purchase-recovery|PurchaseRecovery)/, ['purchase']],
  [/^web\/fixtures\/(?:payment-return|PaymentReturn)/, ['receipt']],
  [/^web\/fixtures\/(?:operation-resolution|OperationResolution)/, ['operation']],
  [/^web\/fixtures\/(?:ip-|IP)/, ['ip']],
  [/^web\/fixtures\/(?:pricing-audit|PricingAudit|NodeStatistics)/, ['charts']],
  [/^web\/fixtures\/(?:backup-recovery|BackupRecovery)/, ['backups']],
  [/^web\/fixtures\/(?:database-audit|DatabaseAudit)/, ['admin-database']],
  [/^web\/fixtures\/(?:coupon-recovery|CouponRecovery)/, ['admin-coupons']],
  [/^web\/fixtures\/(?:emby-connection|EmbyConnection|stablecoin|Stablecoin)/, ['admin-settings']],
  [/^web\/fixtures\/(?:lucky-admin|LuckyAdmin)/, ['admin-activity']],
  [/^web\/fixtures\/(?:lucky-audit|LuckyAudit)/, ['activity']],
  [/^web\/fixtures\/(?:pm-admin|PMRecovery)/, ['pm']],
  [/^web\/fixtures\/(?:xnode|XNode)/, ['xnode-availability', 'xnode-creation']],
  [/^web\/fixtures\/(?:diy|DIY|routing|Routing)/, ['member', 'demonstrations']],
  [/^web\/fixtures\/(?:community|Community|onboarding|Onboarding|detector-retirement)/, ['member']],
]

export function selectBrowserRecovery(changedPaths) {
  const reasons = []
  const selected = new Set()
  for (const path of [...new Set(changedPaths)].sort()) {
    if (typeof path !== 'string' || !path || path.startsWith('/') || path.split('/').includes('..')) throw new Error('Invalid changed path')
    if (/\.(?:md|test\.[cm]?[jt]sx?|spec\.[cm]?[jt]sx?)$/.test(path)) continue
    if (/^web\/public\/fonts\/[^/]+(?:-OFL\.txt|-license\.json)$/.test(path)) continue
    if (/\.(?:regressions\.mjs)$/.test(path)) continue
    const direct = Object.entries(browserRecoverySuites).filter(([, value]) => path === `web/scripts/${value.module.slice(2)}`).map(([key]) => key)
    const rule = rules.find(([pattern]) => pattern.test(path))
    const scopes = direct.length ? direct : rule?.[1]
      ?? (/^(?:web\/|api\/|scripts\/ci\/|\.github\/workflows\/ci\.yml$)/.test(path) ? allBrowserScopes : [])
    if (!scopes.length) continue
    scopes.forEach(scope => selected.add(scope))
    reasons.push({ path, scopes })
  }
  return { scopes: allBrowserScopes.filter(scope => selected.has(scope)), reasons }
}
