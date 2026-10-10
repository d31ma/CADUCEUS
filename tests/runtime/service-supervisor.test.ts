import { describe, expect, test } from 'bun:test'
import {
  PeriodicWorker,
  ServiceSupervisor,
} from '../../server/platform/runtime/service-supervisor.ts'
import { OutboundQueueWorker } from '../../server/features/delivery/services/outbound-queue-worker.ts'
import { FollowUpReminderWorker } from '../../server/features/delivery/services/follow-up-reminder-worker.ts'
import { SnoozeWorker } from '../../server/features/mailbox/services/snooze-worker.ts'
import { ProviderNotificationWorker } from '../../server/features/notifications/services/provider-notification-worker.ts'

describe('ServiceSupervisor', () => {
  test('starts components in order and stops them in reverse order', async () => {
    const events: string[] = []
    const supervisor = new ServiceSupervisor()

    supervisor.register({
      name: 'api-dependent-worker',
      start: async () => {
        events.push('start:worker')
        return async () => { events.push('stop:worker') }
      },
    })
    supervisor.register({
      name: 'smtp',
      start: async () => {
        events.push('start:smtp')
        return async () => { events.push('stop:smtp') }
      },
    })

    await supervisor.start()
    expect(supervisor.snapshot().state).toBe('ready')

    await supervisor.stop()
    await supervisor.stop()

    expect(events).toEqual([
      'start:worker',
      'start:smtp',
      'stop:smtp',
      'stop:worker',
    ])
    expect(supervisor.snapshot().state).toBe('stopped')
  })

  test('rolls back started components when a required component fails', async () => {
    const events: string[] = []
    const supervisor = new ServiceSupervisor()

    supervisor.register({
      name: 'first',
      start: async () => {
        events.push('start:first')
        return async () => { events.push('stop:first') }
      },
    })
    supervisor.register({
      name: 'broken',
      start: async () => {
        events.push('start:broken')
        throw new Error('address already in use')
      },
    })

    await expect(supervisor.start()).rejects.toThrow('address already in use')
    expect(events).toEqual(['start:first', 'start:broken', 'stop:first'])

    const snapshot = supervisor.snapshot()
    expect(snapshot.state).toBe('failed')
    expect(snapshot.components.broken).toMatchObject({
      state: 'failed',
      required: true,
      error: 'address already in use',
    })
  })

  test('reports an optional component failure as degraded without blocking readiness', async () => {
    const supervisor = new ServiceSupervisor()
    supervisor.register({
      name: 'certificate-renewal',
      required: false,
      start: async () => {
        throw new Error('ACME unavailable')
      },
    })
    supervisor.register({
      name: 'smtp',
      start: async () => undefined,
    })

    await supervisor.start()

    const snapshot = supervisor.snapshot()
    expect(snapshot.state).toBe('degraded')
    expect(snapshot.components['certificate-renewal']).toMatchObject({
      state: 'failed',
      required: false,
      error: 'ACME unavailable',
    })
    expect(snapshot.components.smtp.state).toBe('ready')
  })

  test('notifies the runtime when a required running component fails', async () => {
    const failures: Array<{ name: string; message: string }> = []
    let stopped = false
    const supervisor = new ServiceSupervisor({
      onRequiredFailure: (name, error) => {
        failures.push({ name, message: error.message })
      },
    })
    supervisor.register({
      name: 'smtp',
      start: async () => async () => { stopped = true },
    })

    await supervisor.start()
    supervisor.reportFailure('smtp', new Error('listener closed'))
    supervisor.reportFailure('smtp', new Error('duplicate notification'))
    await supervisor.stop()

    expect(failures).toEqual([{ name: 'smtp', message: 'listener closed' }])
    expect(supervisor.snapshot().state).toBe('failed')
    expect(stopped).toBe(true)
  })
})

describe('PeriodicWorker', () => {
  test('prevents overlapping runs and records operational measurements', async () => {
    let release!: () => void
    const blocked = new Promise<void>((resolve) => { release = resolve })
    let calls = 0
    const worker = new PeriodicWorker('outbound-queue', 60_000, async () => {
      calls += 1
      await blocked
    })

    const first = worker.runNow()
    const overlapping = await worker.runNow()

    expect(overlapping).toBe(false)
    expect(worker.snapshot()).toMatchObject({
      name: 'outbound-queue',
      inFlight: true,
      runs: 1,
      skipped: 1,
    })

    release()
    expect(await first).toBe(true)
    expect(calls).toBe(1)
    expect(worker.snapshot()).toMatchObject({
      inFlight: false,
      succeeded: 1,
      failed: 0,
    })
  })

  test('records task failures without creating an unhandled interval rejection', async () => {
    const worker = new PeriodicWorker('outbound-queue', 60_000, async () => {
      throw new Error('API unavailable')
    })

    expect(await worker.runNow()).toBe(false)
    expect(worker.snapshot()).toMatchObject({
      runs: 1,
      succeeded: 0,
      failed: 1,
      lastError: 'API unavailable',
    })
  })

  test('waits for an in-flight task during shutdown', async () => {
    let release!: () => void
    const blocked = new Promise<void>((resolve) => { release = resolve })
    const worker = new PeriodicWorker('outbound-queue', 60_000, () => blocked)
    worker.start()
    const activeRun = worker.runNow()
    let stopped = false

    const stopping = worker.stop().then(() => { stopped = true })
    await Promise.resolve()
    expect(stopped).toBe(false)

    release()
    await activeRun
    await stopping

    expect(stopped).toBe(true)
    expect(worker.snapshot().running).toBe(false)
  })
})

describe('OutboundQueueWorker', () => {
  test('signs queue ticks and exposes the worker measurements', async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = []
    const worker = new OutboundQueueWorker({
      apiUrl: 'http://127.0.0.1:8080/',
      secret: 'queue-secret',
      intervalMs: 60_000,
      fetch: async (input, init) => {
        requests.push({ url: String(input), init })
        return Response.json({ delivered: 2, retried: 1, bounced: 0, queue: { depth: 3, queued: 2, sending: 1, due: 1, oldestAgeMs: 4500, attempts: 4 } })
      },
    })

    expect(await worker.runNow()).toBe(true)
    expect(requests).toHaveLength(1)
    expect(requests[0].url).toBe('http://127.0.0.1:8080/send/tick')
    expect(requests[0].init?.method).toBe('POST')
    expect(new Headers(requests[0].init?.headers).get('X-Caduceus-Signature')).toMatch(/^[a-f0-9]{64}$/)
    expect(worker.snapshot()).toMatchObject({
      name: 'outbound-queue',
      runs: 1,
      succeeded: 1,
      failed: 0,
      lastDelivery: { delivered: 2, retried: 1, bounced: 0 },
      queue: { depth: 3, due: 1, oldestAgeMs: 4500 },
    })
  })

  test('records non-success API responses as failed ticks', async () => {
    const worker = new OutboundQueueWorker({
      apiUrl: 'http://127.0.0.1:8080',
      secret: 'queue-secret',
      intervalMs: 60_000,
      fetch: async () => new Response('{}', { status: 503 }),
    })

    expect(await worker.runNow()).toBe(false)
    expect(worker.snapshot()).toMatchObject({
      failed: 1,
      lastError: 'Queue tick failed with HTTP 503',
    })
  })
})

describe('SnoozeWorker', () => {
  test('signs due-processing ticks and exposes measurements', async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = []
    const worker = new SnoozeWorker({
      apiUrl: 'http://127.0.0.1:8080/', secret: 'snooze-secret', intervalMs: 60_000,
      fetch: async (input, init) => {
        requests.push({ url: String(input), init })
        return new Response('{}', { status: 200 })
      },
    })
    expect(await worker.runNow()).toBe(true)
    expect(requests[0].url).toBe('http://127.0.0.1:8080/snooze/tick')
    expect(new Headers(requests[0].init?.headers).get('X-Caduceus-Signature')).toMatch(/^[a-f0-9]{64}$/)
    expect(worker.snapshot()).toMatchObject({ name: 'snooze-due', runs: 1, succeeded: 1, failed: 0 })
  })

  test('records non-success due-processing responses', async () => {
    const worker = new SnoozeWorker({
      apiUrl: 'http://127.0.0.1:8080', secret: 'snooze-secret', intervalMs: 60_000,
      fetch: async () => new Response('{}', { status: 503 }),
    })
    expect(await worker.runNow()).toBe(false)
    expect(worker.snapshot()).toMatchObject({ failed: 1, lastError: 'Snooze tick failed with HTTP 503' })
  })
})

describe('FollowUpReminderWorker', () => {
  test('signs due-processing ticks and retains the last bounded result', async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = []
    const worker = new FollowUpReminderWorker({
      apiUrl: 'http://127.0.0.1:8080/', secret: 'reminder-secret', intervalMs: 60_000,
      fetch: async (input, init) => {
        requests.push({ url: String(input), init })
        return Response.json({ due: 2, surfaced: 1, reconciled: 1, failed: 0 })
      },
    })
    expect(await worker.runNow()).toBe(true)
    expect(requests[0].url).toBe('http://127.0.0.1:8080/reminders/tick')
    expect(new Headers(requests[0].init?.headers).get('X-Caduceus-Signature')).toMatch(/^[a-f0-9]{64}$/)
    expect(worker.snapshot()).toMatchObject({
      name: 'follow-up-reminders', runs: 1, succeeded: 1, failed: 0,
      lastResult: { due: 2, surfaced: 1, reconciled: 1, failed: 0 },
    })
  })

  test('fails measurements on HTTP errors and malformed result bodies', async () => {
    const unavailable = new FollowUpReminderWorker({
      apiUrl: 'http://127.0.0.1:8080', secret: 'reminder-secret', intervalMs: 60_000,
      fetch: async () => new Response('{}', { status: 503 }),
    })
    expect(await unavailable.runNow()).toBe(false)
    expect(unavailable.snapshot()).toMatchObject({ failed: 1, lastError: 'Reminder tick failed with HTTP 503' })

    const malformed = new FollowUpReminderWorker({
      apiUrl: 'http://127.0.0.1:8080', secret: 'reminder-secret', intervalMs: 60_000,
      fetch: async () => Response.json({ due: -1 }),
    })
    expect(await malformed.runNow()).toBe(false)
    expect(malformed.snapshot()).toMatchObject({ failed: 1, lastError: 'Reminder tick returned an invalid result' })
  })
})

describe('ProviderNotificationWorker', () => {
  test('signs durable provider notification ticks and records measurements', async () => {
    const requests: Array<{ url: string; init?: RequestInit }> = []
    const worker = new ProviderNotificationWorker({
      apiUrl: 'http://127.0.0.1:8080/', secret: 'provider-secret', intervalMs: 60_000,
      fetch: async (input, init) => {
        requests.push({ url: String(input), init })
        return Response.json({ claimed: 1, delivered: 1, retried: 0 })
      },
    })
    expect(await worker.runNow()).toBe(true)
    expect(requests[0].url).toBe('http://127.0.0.1:8080/notifications/provider/tick')
    expect(new Headers(requests[0].init?.headers).get('X-Caduceus-Signature')).toMatch(/^[a-f0-9]{64}$/)
    expect(worker.snapshot()).toMatchObject({ name: 'provider-new-mail', runs: 1, succeeded: 1, failed: 0 })
  })

  test('records a non-success provider notification tick', async () => {
    const worker = new ProviderNotificationWorker({
      apiUrl: 'http://127.0.0.1:8080', secret: 'provider-secret', intervalMs: 60_000,
      fetch: async () => new Response('{}', { status: 503 }),
    })
    expect(await worker.runNow()).toBe(false)
    expect(worker.snapshot()).toMatchObject({ failed: 1, lastError: 'Provider notification tick failed with HTTP 503' })
  })
})
