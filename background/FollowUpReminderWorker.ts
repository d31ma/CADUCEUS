import { createHmac } from 'node:crypto'
import {
  PeriodicWorker,
  type StopService,
  type WorkerSnapshot,
} from './ServiceSupervisor.ts'

export type ReminderFetchRequest = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>

export interface FollowUpReminderWorkerOptions {
  apiUrl: string
  secret: string
  intervalMs?: number
  timeoutMs?: number
  fetch?: ReminderFetchRequest
}

/** Drives restart-safe reminder resurfacing while the API owns FYLO writes. */
export class FollowUpReminderWorker {
  readonly #worker: PeriodicWorker
  #lastResult?: Record<string, number>

  constructor(options: FollowUpReminderWorkerOptions) {
    const apiUrl = options.apiUrl.replace(/\/$/, '')
    const intervalMs = options.intervalMs ?? 30_000
    const timeoutMs = options.timeoutMs ?? 60_000
    const fetchRequest = options.fetch ?? globalThis.fetch
    const body = '{}'

    this.#worker = new PeriodicWorker('follow-up-reminders', intervalMs, async () => {
      const signature = createHmac('sha256', options.secret).update(body).digest('hex')
      const response = await fetchRequest(`${apiUrl}/reminders/tick`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Caduceus-Signature': signature,
        },
        body,
        signal: AbortSignal.timeout(timeoutMs),
      })
      if (!response.ok) throw new Error(`Reminder tick failed with HTTP ${response.status}`)
      const result: unknown = await response.json()
      if (!isReminderResult(result)) throw new Error('Reminder tick returned an invalid result')
      this.#lastResult = result
    })
  }

  start(): StopService { return this.#worker.start() }
  stop(): Promise<void> { return this.#worker.stop() }
  runNow(): Promise<boolean> { return this.#worker.runNow() }
  snapshot(): WorkerSnapshot & { lastResult?: Record<string, number> } {
    return { ...this.#worker.snapshot(), ...(this.#lastResult ? { lastResult: this.#lastResult } : {}) }
  }
}

function isReminderResult(value: unknown): value is Record<string, number> {
  if (!value || typeof value !== 'object') return false
  return ['due', 'surfaced', 'reconciled', 'failed'].every(key => {
    const count = (value as Record<string, unknown>)[key]
    return Number.isSafeInteger(count) && Number(count) >= 0
  })
}
