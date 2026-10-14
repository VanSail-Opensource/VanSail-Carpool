import type { Session } from './types'
import { request } from './http'

export interface BrowserConfig { miniAppUrl: string }
export interface AdminAccessLink { url: string; expiresAt: string }
export type BrowserSession =
  | (Session & { kind: 'admin'; expiresAt: string; returnTo?: string })
  | { authenticated: false; kind: 'anonymous'; miniAppUrl: string }
  | { authenticated: false; kind: 'member'; miniAppUrl: string; user: { telegramId: string; firstName: string; lastName: string; telegramUsername: string } }

export const browserAccessApi = {
  config: () => request<BrowserConfig>('/api/v1/browser/config'),
  session: () => request<BrowserSession>('/api/v1/auth/browser/session'),
  renew: () => request<BrowserSession>('/api/v1/auth/browser/session/renew', { method: 'POST' }),
  logout: () => request<void>('/api/v1/auth/browser/logout', { method: 'POST' }),
  exchange: (token: string) => request<BrowserSession>('/api/v1/auth/browser/exchange', { method: 'POST', body: { token } }),
  createLink: (returnTo?: string) => request<AdminAccessLink>('/api/v1/admin-access/links', { method: 'POST', body: { returnTo } }),
}
