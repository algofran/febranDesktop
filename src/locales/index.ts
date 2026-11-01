// One dictionary file per screen, per language: src/locales/<lang>/<screen>.ts.
// Keys are looked up as "<screen>.<key>"; a missing Indonesian key falls back to English.
import enNavbar from './en/navbar'
import enSettings from './en/settings'
import enDashboard from './en/dashboard'
import enSessions from './en/sessions'
import enSandbox from './en/sandbox'
import enLogs from './en/logs'
import enService from './en/service'
import enApp from './en/app'
import idNavbar from './id/navbar'
import idSettings from './id/settings'
import idDashboard from './id/dashboard'
import idSessions from './id/sessions'
import idSandbox from './id/sandbox'
import idLogs from './id/logs'
import idService from './id/service'
import idApp from './id/app'

export type Locale = 'en' | 'id'
export type Dict = Record<string, unknown>

export const LOCALES: { code: Locale; label: string }[] = [
  { code: 'en', label: 'English' },
  { code: 'id', label: 'Indonesia' },
]

export const messages: Record<Locale, Dict> = {
  en: { navbar: enNavbar, settings: enSettings, dashboard: enDashboard, sessions: enSessions, sandbox: enSandbox, logs: enLogs, service: enService, app: enApp },
  id: { navbar: idNavbar, settings: idSettings, dashboard: idDashboard, sessions: idSessions, sandbox: idSandbox, logs: idLogs, service: idService, app: idApp },
}
