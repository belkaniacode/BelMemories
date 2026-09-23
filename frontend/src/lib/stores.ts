import { writable } from 'svelte/store'
import type { ClassifierStatus, Progress, Report, ScanStats, ScanSummary } from './api'

export type Step = 'source' | 'scan' | 'destination' | 'progress' | 'report'

export const step = writable<Step>('source')

/** Selected source folders/disks. */
export const sources = writable<string[]>([])

/** Live scan state. */
export const scanning = writable(false)
export const scanStats = writable<ScanStats | null>(null)
export const scanSummary = writable<ScanSummary | null>(null)

/** Archive run state. */
export const destination = writable<string>('')
export const archiving = writable(false)
export const progress = writable<Progress | null>(null)
export const report = writable<Report | null>(null)

export const classifier = writable<ClassifierStatus | null>(null)

/** Global toast-like error line. */
export const errorMessage = writable<string>('')

/** Parameters of the last archive run (lets the plan screen start the real run). */
export interface ArchiveRequest {
  root: string
  dryRun: boolean
  verify: boolean
  load: string
}
export const lastRequest = writable<ArchiveRequest | null>(null)

/** Colour scheme shown in the UI. The "auto" setting is resolved to one of these at startup. */
export type Theme = 'light' | 'dark'
export const theme = writable<Theme>('light')
