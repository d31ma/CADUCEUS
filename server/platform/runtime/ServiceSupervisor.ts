export type RuntimeState =
  | 'idle'
  | 'starting'
  | 'ready'
  | 'degraded'
  | 'stopping'
  | 'stopped'
  | 'failed'

export type ComponentState =
  | 'pending'
  | 'starting'
  | 'ready'
  | 'stopping'
  | 'stopped'
  | 'failed'

export type StopService = () => void | Promise<void>

export interface ServiceDefinition {
  name: string
  required?: boolean
  start: () => void | StopService | Promise<void | StopService>
}

export interface ComponentSnapshot {
  required: boolean
  state: ComponentState
  startedAt?: string
  stoppedAt?: string
  error?: string
}

export interface SupervisorSnapshot {
  state: RuntimeState
  startedAt?: string
  readyAt?: string
  stoppedAt?: string
  components: Record<string, ComponentSnapshot>
}

export interface ServiceSupervisorOptions {
  onRequiredFailure?: (name: string, error: Error) => void | Promise<void>
}

interface RegisteredService {
  definition: Required<Pick<ServiceDefinition, 'name' | 'start'>> & {
    required: boolean
  }
  snapshot: ComponentSnapshot
  stop?: StopService
}

/**
 * Owns a group of process-level services as one lifecycle.
 *
 * Required startup failures roll back already-started services. Optional
 * failures leave the runtime degraded but ready, which is useful for helpers
 * such as certificate renewal that should not hide an otherwise usable server.
 */
export class ServiceSupervisor {
  readonly #services: RegisteredService[] = []
  readonly #onRequiredFailure?: ServiceSupervisorOptions['onRequiredFailure']
  #state: RuntimeState = 'idle'
  #startedAt?: string
  #readyAt?: string
  #stoppedAt?: string
  #stopPromise?: Promise<void>

  constructor(options: ServiceSupervisorOptions = {}) {
    this.#onRequiredFailure = options.onRequiredFailure
  }

  register(definition: ServiceDefinition): this {
    if (this.#state !== 'idle') {
      throw new Error('Services must be registered before the supervisor starts')
    }
    if (!definition.name.trim()) {
      throw new Error('A supervised service requires a name')
    }
    if (this.#services.some((service) => service.definition.name === definition.name)) {
      throw new Error(`Service "${definition.name}" is already registered`)
    }

    this.#services.push({
      definition: {
        name: definition.name,
        required: definition.required !== false,
        start: definition.start,
      },
      snapshot: {
        required: definition.required !== false,
        state: 'pending',
      },
    })
    return this
  }

  async start(): Promise<void> {
    if (this.#state !== 'idle') {
      throw new Error(`Cannot start services while supervisor is ${this.#state}`)
    }

    this.#state = 'starting'
    this.#startedAt = new Date().toISOString()
    let degraded = false

    for (const service of this.#services) {
      service.snapshot.state = 'starting'
      try {
        const stop = await service.definition.start()
        service.stop = typeof stop === 'function' ? stop : undefined
        service.snapshot.state = 'ready'
        service.snapshot.startedAt = new Date().toISOString()
      } catch (cause) {
        const error = ServiceSupervisor.toError(cause)
        service.snapshot.state = 'failed'
        service.snapshot.error = error.message

        if (!service.definition.required) {
          degraded = true
          continue
        }

        this.#state = 'failed'
        await this.#stopStartedServices()
        throw error
      }
    }

    this.#readyAt = new Date().toISOString()
    this.#state = degraded ? 'degraded' : 'ready'
  }

  async stop(): Promise<void> {
    if (this.#stopPromise) return this.#stopPromise
    if (this.#state === 'idle' || this.#state === 'stopped') {
      this.#state = 'stopped'
      this.#stoppedAt ??= new Date().toISOString()
      return
    }

    this.#stopPromise = (async () => {
      const terminalState = this.#state === 'failed' ? 'failed' : 'stopped'
      this.#state = 'stopping'
      await this.#stopStartedServices()
      this.#stoppedAt = new Date().toISOString()
      this.#state = terminalState
    })()
    return this.#stopPromise
  }

  reportFailure(name: string, cause: unknown): void {
    const service = this.#services.find((candidate) => candidate.definition.name === name)
    if (!service) throw new Error(`Unknown supervised service "${name}"`)
    if (
      service.snapshot.state === 'failed'
      || service.snapshot.state === 'stopping'
      || service.snapshot.state === 'stopped'
    ) return

    const error = ServiceSupervisor.toError(cause)
    service.snapshot.state = 'failed'
    service.snapshot.error = error.message

    if (!service.definition.required) {
      if (this.#state === 'ready') this.#state = 'degraded'
      return
    }

    this.#state = 'failed'
    if (this.#onRequiredFailure) {
      Promise.resolve(this.#onRequiredFailure(name, error)).catch(() => {
        // Failure reporting must not create an unhandled rejection while the
        // process is already following its fatal-shutdown path.
      })
    }
  }

  snapshot(): SupervisorSnapshot {
    return {
      state: this.#state,
      ...(this.#startedAt ? { startedAt: this.#startedAt } : {}),
      ...(this.#readyAt ? { readyAt: this.#readyAt } : {}),
      ...(this.#stoppedAt ? { stoppedAt: this.#stoppedAt } : {}),
      components: Object.fromEntries(
        this.#services.map((service) => [
          service.definition.name,
          { ...service.snapshot },
        ]),
      ),
    }
  }

  async #stopStartedServices(): Promise<void> {
    for (const service of [...this.#services].reverse()) {
      if (
        !service.stop
        || (service.snapshot.state !== 'ready' && service.snapshot.state !== 'failed')
      ) continue

      service.snapshot.state = 'stopping'
      try {
        await service.stop()
        service.snapshot.state = 'stopped'
        service.snapshot.stoppedAt = new Date().toISOString()
      } catch (cause) {
        service.snapshot.state = 'failed'
        service.snapshot.error = ServiceSupervisor.toError(cause).message
      }
    }
  }

  static toError(cause: unknown): Error {
    return cause instanceof Error ? cause : new Error(String(cause))
  }
}

export interface WorkerSnapshot {
  name: string
  running: boolean
  inFlight: boolean
  runs: number
  succeeded: number
  failed: number
  skipped: number
  lastStartedAt?: string
  lastCompletedAt?: string
  lastDurationMs?: number
  lastError?: string
}

/**
 * A non-overlapping periodic task with a small, serializable metrics surface.
 */
export class PeriodicWorker {
  readonly #name: string
  readonly #intervalMs: number
  readonly #task: () => void | Promise<void>
  #timer?: ReturnType<typeof setInterval>
  #activeRun?: Promise<boolean>
  #runs = 0
  #succeeded = 0
  #failed = 0
  #skipped = 0
  #lastStartedAt?: string
  #lastCompletedAt?: string
  #lastDurationMs?: number
  #lastError?: string

  constructor(
    name: string,
    intervalMs: number,
    task: () => void | Promise<void>,
  ) {
    if (!name.trim()) throw new Error('A periodic worker requires a name')
    if (!Number.isFinite(intervalMs) || intervalMs <= 0) {
      throw new Error('A periodic worker interval must be greater than zero')
    }
    this.#name = name
    this.#intervalMs = intervalMs
    this.#task = task
  }

  start(): StopService {
    if (this.#timer) throw new Error(`Periodic worker "${this.#name}" is already running`)
    this.#timer = setInterval(() => {
      void this.runNow()
    }, this.#intervalMs)
    this.#timer.unref?.()
    return () => this.stop()
  }

  async stop(): Promise<void> {
    if (this.#timer) {
      clearInterval(this.#timer)
      this.#timer = undefined
    }
    await this.#activeRun
  }

  runNow(): Promise<boolean> {
    if (this.#activeRun) {
      this.#skipped += 1
      return Promise.resolve(false)
    }

    const activeRun = this.#execute()
    const trackedRun = activeRun.finally(() => {
      if (this.#activeRun === trackedRun) this.#activeRun = undefined
    })
    this.#activeRun = trackedRun
    return trackedRun
  }

  async #execute(): Promise<boolean> {
    this.#runs += 1
    this.#lastStartedAt = new Date().toISOString()
    const started = performance.now()

    try {
      await this.#task()
      this.#succeeded += 1
      this.#lastError = undefined
      return true
    } catch (cause) {
      this.#failed += 1
      this.#lastError = ServiceSupervisor.toError(cause).message
      return false
    } finally {
      this.#lastCompletedAt = new Date().toISOString()
      this.#lastDurationMs = Math.round((performance.now() - started) * 100) / 100
    }
  }

  snapshot(): WorkerSnapshot {
    return {
      name: this.#name,
      running: Boolean(this.#timer),
      inFlight: Boolean(this.#activeRun),
      runs: this.#runs,
      succeeded: this.#succeeded,
      failed: this.#failed,
      skipped: this.#skipped,
      ...(this.#lastStartedAt ? { lastStartedAt: this.#lastStartedAt } : {}),
      ...(this.#lastCompletedAt ? { lastCompletedAt: this.#lastCompletedAt } : {}),
      ...(this.#lastDurationMs !== undefined ? { lastDurationMs: this.#lastDurationMs } : {}),
      ...(this.#lastError ? { lastError: this.#lastError } : {}),
    }
  }
}
