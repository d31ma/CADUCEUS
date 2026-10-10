import { createHmac } from 'node:crypto'
import {
  PeriodicWorker,
  type StopService,
  type WorkerSnapshot,
} from '../../../platform/runtime/ServiceSupervisor.ts'

export interface ProviderNotificationWorkerOptions {
  apiUrl: string
  secret: string
  intervalMs?: number
  timeoutMs?: number
  fetch?: (input: RequestInfo | URL, init?: RequestInit) => Promise<Response>
}

/** Drives the durable provider new-mail outbox through a signed local route. */
export class ProviderNotificationWorker {
  readonly #worker: PeriodicWorker

  constructor(options: ProviderNotificationWorkerOptions) {
    const apiUrl = options.apiUrl.replace(/\/$/, '')
    const body = '{}'
    const fetchRequest = options.fetch ?? globalThis.fetch
    this.#worker = new PeriodicWorker('provider-new-mail', options.intervalMs ?? 15_000, async () => {
      const signature = createHmac('sha256', options.secret).update(body).digest('hex')
      const response = await fetchRequest(`${apiUrl}/notifications/provider/tick`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', 'X-Caduceus-Signature': signature },
        body,
        signal: AbortSignal.timeout(options.timeoutMs ?? 60_000),
      })
      if (!response.ok) throw new Error(`Provider notification tick failed with HTTP ${response.status}`)
    })
  }

  start(): StopService { return this.#worker.start() }
  stop(): Promise<void> { return this.#worker.stop() }
  runNow(): Promise<boolean> { return this.#worker.runNow() }
  snapshot(): WorkerSnapshot { return this.#worker.snapshot() }
}
