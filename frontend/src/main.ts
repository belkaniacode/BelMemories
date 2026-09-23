import './style.css'
import { mount } from 'svelte'
import App from './App.svelte'
import { reportError } from './lib/api'

window.addEventListener('error', (e) => reportError(e.error ?? e.message))
window.addEventListener('unhandledrejection', (e) => reportError(e.reason))

const app = mount(App, { target: document.getElementById('app')! })

export default app
