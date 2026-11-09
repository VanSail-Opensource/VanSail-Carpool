import { t } from '@/i18n'

export function endpointFieldLabel(field: string): string {
  const key = `endpointFields.${field}`, value = t(key)
  return value === key ? t(`diyFields.${field}`) : value
}
export function endpointError(code: string): string {
  const key = `endpointOptions.errors.${code}`, value = t(key)
  return value === key ? t('endpointOptions.errors.invalid') : value
}
