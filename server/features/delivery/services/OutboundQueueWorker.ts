import { createHmac } from 'node:crypto'
import {
  PeriodicWorker,
  type StopService,
  type WorkerSnapshot,
} from '../../../platform/runtime/ServiceSupervisor.ts'

export type FetchRequest = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>

export interface OutboundQueueWorkerOptions {
  apiUrl: string
  secret: string
  intervalMs?: number
  timeoutMs?: number
  fetch?: FetchRequest
}

export interface QueueHealth {
  depth: number
  queued: number
  sending: number
  due: number
  oldestAgeMs: number
  attempts: number
}

export interface OutboundQueueSnapshot extends WorkerSnapshot {
  lastDelivery?: { delivered: number; retried: number; bounced: number }
  queue?: QueueHealth
}

/**
 * Drives the durable outbound queue through the local Tachyon API.
 *
 * The API remains the only Fylo writer. This worker owns scheduling,
 * non-overlap, request signing, and measurements, but no delivery state.
 */
export class OutboundQueueWorker {
  readonly #worker: PeriodicWorker
  #lastDelivery?: { delivered: number; retried: number; bounced: number }
  #queue?: QueueHealth

  constructor(options: OutboundQueueWorkerOptions) {
    const apiUrl = options.apiUrl.replace(/\/$/, '')
    const intervalMs = options.intervalMs ?? 30_000
    const timeoutMs = options.timeoutMs ?? 60_000
    const fetchRequest = options.fetch ?? globalThis.fetch
    const body = '{}'

    this.#worker = new PeriodicWorker('outbound-queue', intervalMs, async () => {
      const signature = createHmac('sha256', options.secret).update(body).digest('hex')
      const response = await fetchRequest(`${apiUrl}/send/tick`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Caduceus-Signature': signature,
        },
        body,
        signal: AbortSignal.timeout(timeoutMs),
      })
      if (!response.ok) {
        throw new Error(`Queue tick failed with HTTP ${response.status}`)
      }
      const result = await response.json() as Record<string, unknown>
      this.#lastDelivery = {
        delivered: boundedCount(result.delivered), retried: boundedCount(result.retried), bounced: boundedCount(result.bounced),
      }
      this.#queue = queueHealth(result.queue)
    })
  }

  start(): StopService {
    return this.#worker.start()
  }

  stop(): Promise<void> {
    return this.#worker.stop()
  }

  runNow(): Promise<boolean> {
    return this.#worker.runNow()
  }

  snapshot(): OutboundQueueSnapshot {
    return { ...this.#worker.snapshot(), ...(this.#lastDelivery ? { lastDelivery: this.#lastDelivery } : {}), ...(this.#queue ? { queue: this.#queue } : {}) }
  }
}

function boundedCount(value: unknown): number {
  const count = Number(value)
  return Number.isInteger(count) && count >= 0 && count <= Number.MAX_SAFE_INTEGER ? count : 0
}

function queueHealth(value: unknown): QueueHealth | undefined {
  if (!value || typeof value !== 'object') return undefined
  const input = value as Record<string, unknown>
  return {
    depth: boundedCount(input.depth), queued: boundedCount(input.queued), sending: boundedCount(input.sending),
    due: boundedCount(input.due), oldestAgeMs: boundedCount(input.oldestAgeMs), attempts: boundedCount(input.attempts),
  }
}
