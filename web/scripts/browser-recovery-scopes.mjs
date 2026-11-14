// Each PR selects these stable identifiers; the runner imports only its selected suites.
const suite = (module, exported) => ({ module: `./${module}.mjs`, export: exported })
export const browserRecoverySuites = Object.freeze({
  currency: suite('browser-core-recovery', 'currencyRecoveryScenarios'),
  checkout: suite('browser-core-recovery', 'checkoutRecoveryScenarios'),
  auth: suite('browser-core-recovery', 'authRecoveryScenarios'),
  member: suite('browser-member-recovery', 'memberRecoveryScenarios'),
  operation: suite('browser-operation-recovery', 'operationRecoveryScenarios'),
  purchase: suite('browser-purchase-recovery', 'purchaseRecoveryScenarios'),
  activity: suite('browser-activity-recovery', 'activityRecoveryScenarios'),
  receipt: suite('browser-receipt-recovery', 'receiptRecoveryScenarios'),
  hourly: suite('browser-hourly-recovery', 'hourlyRecoveryScenarios'),
  ip: suite('browser-ip-recovery', 'ipRecoveryScenarios'),
  settings: suite('browser-settings-recovery', 'settingsRecoveryScenarios'),
  charts: suite('browser-chart-recovery', 'chartPrecisionScenarios'),
  'admin-settings': suite('browser-admin-settings-recovery', 'adminSettingsRecoveryScenarios'),
  backups: suite('browser-backup-recovery', 'backupRecoveryScenarios'),
  'admin-activity': suite('browser-admin-activity-recovery', 'adminActivityRecoveryScenarios'),
  'admin-coupons': suite('browser-admin-coupon-recovery', 'adminCouponRecoveryScenarios'),
  'admin-database': suite('browser-admin-database-recovery', 'adminDatabaseRecoveryScenarios'),
  pm: suite('browser-pm-recovery', 'pmRecoveryScenarios'),
  'xnode-availability': suite('browser-xnode-availability', 'xnodeAvailabilityScenarios'),
  'xnode-creation': suite('browser-xnode-creation', 'xnodeCreationScenarios'),
  demonstrations: suite('browser-demonstrations', 'demonstrationScenarios'),
  knowledgebase: suite('browser-knowledgebase', 'knowledgebaseScenarios'),
  'member-design': suite('browser-member-design', 'memberDesignScenarios'),
})
export const allBrowserScopes = Object.keys(browserRecoverySuites)
export const memberBrowserScopes = ['currency', 'checkout', 'member', 'operation', 'purchase', 'activity', 'receipt', 'hourly', 'ip', 'settings', 'charts', 'demonstrations', 'knowledgebase', 'member-design']
export const adminBrowserScopes = ['auth', 'operation', 'ip', 'charts', 'admin-settings', 'backups', 'admin-activity', 'admin-coupons', 'admin-database', 'pm', 'xnode-availability', 'xnode-creation']

// No implicit universal fallback: the PR planning job must provide a reviewed scope.
export function parseBrowserScopes(raw) {
  const scopes = JSON.parse(raw ?? 'null')
  if (!Array.isArray(scopes) || scopes.length === 0 || scopes.some(scope => typeof scope !== 'string' || !Object.hasOwn(browserRecoverySuites, scope))) {
    throw new Error('BROWSER_AUDIT_SCOPES must be a non-empty JSON array of known browser recovery scopes')
  }
  return [...new Set(scopes)]
}
