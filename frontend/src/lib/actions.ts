// Actions shared by several screens.
import { get } from 'svelte/store'
import { Backend, errorText, reportError } from './api'
import { archiving, errorMessage, lastRequest, progress, report, scanStats, scanSummary, sources, step, theme, type ArchiveRequest, type Theme } from './stores'

/** Starts an archive run (or a dry run) and switches to the progress screen. */
export async function startArchive(req: ArchiveRequest): Promise<void> {
  errorMessage.set('')
  progress.set(null)
  report.set(null)
  try {
    await Backend.StartArchive(req)
    lastRequest.set(req)
    archiving.set(true)
    step.set('progress')
  } catch (e) {
    errorMessage.set(errorText(e))
  }
}

/**
 * Starts over: forgets the selected sources, the scan and the last report, so
 * the next run archives only the folders chosen now.
 */
export function newArchive(): void {
  console.log('[FIX] new archive: clearing selected sources', get(sources))
  sources.set([])
  scanStats.set(null)
  scanSummary.set(null)
  progress.set(null)
  report.set(null)
  lastRequest.set(null)
  errorMessage.set('')
  step.set('source')
}

/** Applies the theme to the document (CSS reads html[data-theme]). */
export function applyTheme(t: Theme): void {
  theme.set(t)
  document.documentElement.dataset.theme = t
}

/**
 * Picks the startup theme: the saved choice, or — while the setting is still
 * "auto" — the OS scheme detected once by the backend. It does not follow later
 * OS changes; the user switches with the toolbar button.
 */
export async function initTheme(): Promise<void> {
  let saved = ''
  try {
    saved = (await Backend.GetSettings()).theme
  } catch (e) {
    console.error('[FIX] theme: GetSettings failed', e)
  }
  if (saved === 'light' || saved === 'dark') {
    console.log('[FIX] theme: saved choice', saved)
    applyTheme(saved)
    return
  }
  try {
    const sys = await Backend.SystemTheme()
    console.log('[FIX] theme: detected from OS', sys)
    applyTheme(sys === 'dark' ? 'dark' : 'light')
  } catch (e) {
    const dark = window.matchMedia('(prefers-color-scheme: dark)').matches
    console.error('[FIX] theme: SystemTheme failed, using prefers-color-scheme', { dark, e })
    applyTheme(dark ? 'dark' : 'light')
  }
}

/** Switches light ↔ dark and persists the choice. */
export async function toggleTheme(): Promise<void> {
  const t: Theme = get(theme) === 'dark' ? 'light' : 'dark'
  applyTheme(t)
  try {
    const st = await Backend.GetSettings()
    st.theme = t
    await Backend.SaveSettings(st)
  } catch (e) {
    reportError(e)
  }
}
