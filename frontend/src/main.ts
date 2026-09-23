import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { Backend, reportError } from './lib/api'
import { langFromLocale, setLang, type Lang } from './lib/i18n'
import { initTheme } from './lib/actions'

window.addEventListener('error', (e) => reportError(e.error ?? e.message))
window.addEventListener('unhandledrejection', (e) => reportError(e.reason))

// The language comes from the OS (backend); the browser locale is a fallback.
async function detectLang(): Promise<Lang> {
  try {
    const l = await Backend.GetLanguage()
    if (l === 'ru' || l === 'en') return l
  } catch (e) {
    console.error('[i18n] GetLanguage failed, using navigator.language', e)
  }
  return langFromLocale(navigator.language)
}

// Language and theme are both resolved before the first paint: no flash of
// the light palette for dark-theme users.
const app = Promise.all([detectLang(), initTheme()]).then(([l]) => {
  setLang(l)
  return mount(App, { target: document.getElementById('app')! })
})

export default app
