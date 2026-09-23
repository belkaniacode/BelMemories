// Thin typed layer over the generated Wails bindings plus formatting helpers.
export * as Backend from '../../wailsjs/go/main/App.js'
export { EventsOn } from '../../wailsjs/runtime/runtime.js'
import { LogFrontendError } from '../../wailsjs/go/main/App.js'

export type LoadLevel = 'low' | 'medium' | 'high'

export interface ScanStats {
  photos: number
  videos: number
  photoBytes: number
  videoBytes: number
  skipped: number
  errors: number
  dirs: number
  done: boolean
}

export interface ScanSummary {
  stats: ScanStats
  errorPaths: string[]
  seconds: number
  cancelled: boolean
  roots: string[]
}

export interface Progress {
  phase: string
  total: number
  processed: number
  totalBytes: number
  processedBytes: number
  writtenBytes: number
  bytesPerSec: number
  etaSeconds: number
  copied: number
  duplicates: number
  renamed: number
  errors: number
  noDate: number
  photos: number
  videos: number
  pictures: number
  current: string
  elapsedSeconds: number
}

export interface FileResult {
  src: string
  dst?: string
  action: string
  category?: string
  year?: number
  dateSource?: string
  method?: string
  renamed?: boolean
  dupOf?: string
  err?: string
}

export interface Report {
  runId: number
  root: string
  sources: string[]
  dryRun: boolean
  status: 'completed' | 'cancelled' | 'disk_error' | 'error'
  message?: string
  stats: Progress
  renamed: FileResult[] | null
  errors: FileResult[] | null
  noDate: FileResult[] | null
  mtimeDates: FileResult[] | null
  planned?: FileResult[] | null
  missingArchives?: string[] | null
  reportPath: string
  csvPath: string
}

export interface ClassifierStatus {
  clipAvailable: boolean
  reason: string
  modelDir: string
  libPath: string
}

const units = ['Б', 'КБ', 'МБ', 'ГБ', 'ТБ']

export function fmtBytes(n: number): string {
  if (!n || n < 0) return '0 Б'
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toLocaleString('ru-RU', { maximumFractionDigits: v < 10 && i > 0 ? 1 : 0 })} ${units[i]}`
}

export function fmtNum(n: number): string {
  return (n ?? 0).toLocaleString('ru-RU')
}

export function fmtDuration(sec: number): string {
  if (!sec || sec < 0) return '—'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  if (h > 0) return `${h} ч ${m} мин`
  if (m > 0) return `${m} мин ${s} с`
  return `${s} с`
}

export function errorText(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e)
}

/** Reports a UI error to the backend log without throwing. */
export function reportError(e: unknown): void {
  const msg = errorText(e)
  console.error(msg)
  LogFrontendError(msg).catch(() => {})
}
