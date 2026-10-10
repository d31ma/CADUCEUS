import { createHmac } from 'node:crypto'
import {
  PeriodicWorker,
  type StopService,
  type WorkerSnapshot,
} from '../../../platform/runtime/service-supervisor.ts'

export type SnoozeFetchRequest = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>

export interface SnoozeWorkerOptions {
  apiUrl: string
  secret: string
  intervalMs?: number
  timeoutMs?: number
  fetch?: SnoozeFetchRequest
}

/** Drives durable due-snooze processing while the API remains the sole FYLO writer. */
export class SnoozeWorker {
  readonly #worker: PeriodicWorker

  constructor(options: SnoozeWorkerOptions) {
    const apiUrl = options.apiUrl.replace(/\/$/, '')
    const intervalMs = options.intervalMs ?? 30_000
    const timeoutMs = options.timeoutMs ?? 60_000
    const fetchRequest = options.fetch ?? globalThis.fetch
    const body = '{}'

    this.#worker = new PeriodicWorker('snooze-due', intervalMs, async () => {
      const signature = createHmac('sha256', options.secret).update(body).digest('hex')
      const response = await fetchRequest(`${apiUrl}/snooze/tick`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Caduceus-Signature': signature,
        },
        body,
        signal: AbortSignal.timeout(timeoutMs),
      })
      if (!response.ok) throw new Error(`Snooze tick failed with HTTP ${response.status}`)
    })
  }

  start(): StopService { return this.#worker.start() }
  stop(): Promise<void> { return this.#worker.stop() }
  runNow(): Promise<boolean> { return this.#worker.runNow() }
  snapshot(): WorkerSnapshot { return this.#worker.snapshot() }
}
