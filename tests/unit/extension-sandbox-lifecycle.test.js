import { describe, expect, it } from 'bun:test'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

class SandboxHarness {
  constructor({ failConstruction = false } = {}) {
    this.now = 0
    this.nextTimer = 0
    this.timers = new Map()
    this.workers = []
    this.messages = []
    this.urls = []
    this.revoked = []
    this.parent = { postMessage: message => this.messages.push(message) }
    const harness = this
    class Worker {
      constructor() {
        if (failConstruction) throw new Error('Worker construction failed')
        this.terminated = false
        this.commands = []
        harness.workers.push(this)
      }
      terminate() { this.terminated = true }
      postMessage(message) { this.commands.push(message) }
    }
    runInNewContext(this.script(), {
      Worker,
      Blob: class {},
      URL: {
        createObjectURL: () => { const url = `blob:test-${this.urls.length}`; this.urls.push(url); return url },
        revokeObjectURL: url => this.revoked.push(url),
      },
      parent: this.parent,
      addEventListener: (_name, listener) => { this.receive = listener },
      setTimeout: (callback, delay) => {
        const id = ++this.nextTimer
        this.timers.set(id, { callback, due: this.now + delay })
        return id
      },
      clearTimeout: id => this.timers.delete(id),
    })
    this.messages.length = 0
  }

  script() {
    const file = process.env.CADUCEUS_SANDBOX_SOURCE || new URL('../../client/shared/utils/ExtensionSandboxUtils.go', import.meta.url)
    const source = readFileSync(file, 'utf8')
    const expression = source.split('\tbootstrap := ')[1].split('\n\tpolicy :=')[0].trim()
    const timeout = source.match(/commandTimeoutMs\(\) int \{ return (\d+) \}/)[1]
    const values = new Map([
      ['utils.literal(prefix)', '""'],
      ['utils.literal(suffix)', '""'],
      ['timeout', timeout],
      ['strconv.Itoa(utils.maxResultBytes())', '16384'],
    ])
    return expression.split('`').map((part, index) => {
      if (index % 2) return part
      const name = part.replace(/^\s*\+\s*|\s*\+\s*$/g, '').trim()
      if (!name) return ''
      if (!values.has(name)) throw new Error(`Unknown host interpolation: ${name}`)
      return values.get(name)
    }).join('')
  }

  send(message) {
    this.receive({ source: this.parent, data: { channel: 'caduceus-extension-host', ...message } })
  }

  load(nonce = 'first') {
    this.send({ type: 'load', extensionId: 'plugin', sessionNonce: nonce, source: '' })
    return this.workers.at(-1)
  }

  ready(worker) { worker.onmessage({ data: { type: 'ready' } }) }

  command(nonce = 'first', requestId = 'request') {
    this.send({ type: 'command', extensionId: 'plugin', sessionNonce: nonce, requestId, command: {}, context: {} })
  }

  advance(milliseconds) {
    const end = this.now + milliseconds
    while (true) {
      const next = [...this.timers].sort((left, right) => left[1].due - right[1].due)[0]
      if (!next || next[1].due > end) break
      this.now = next[1].due
      this.timers.delete(next[0])
      next[1].callback()
    }
    this.now = end
  }

  errors() { return this.messages.filter(message => ['error', 'stopped'].includes(message.type)) }
}

describe('Go extension sandbox worker lifecycle', () => {
  it('reset cancels pending command and activation timers', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.command()
    host.send({ type: 'reset' })
    host.advance(3000)
    expect(worker.terminated).toBe(true)
    expect(host.errors()).toEqual([])
  })

  it('an old command timeout cannot terminate a replacement worker', () => {
    const host = new SandboxHarness()
    const old = host.load()
    host.ready(old)
    host.command()
    host.advance(1000)
    const replacement = host.load('second')
    host.ready(replacement)
    host.advance(2000)
    expect(old.terminated).toBe(true)
    expect(replacement.terminated).toBe(false)
    expect(host.errors()).toEqual([])
    host.command('second', 'replacement-command')
    expect(replacement.commands).toHaveLength(1)
  })

  it('replacing a starting worker prevents stale activation errors', () => {
    const host = new SandboxHarness()
    host.load()
    const replacement = host.load('second')
    host.ready(replacement)
    host.advance(3000)
    expect(replacement.terminated).toBe(false)
    expect(host.errors()).toEqual([])
  })

  it('a timed out worker settles once and accepts no further commands', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.ready(worker)
    host.command('first', 'one')
    host.command('first', 'two')
    host.advance(3000)
    expect(worker.terminated).toBe(true)
    expect(host.errors()).toHaveLength(1)
    expect(host.errors()[0].type).toBe('stopped')
    host.command('first', 'three')
    expect(worker.commands).toHaveLength(2)
  })

  it('activation timeout removes the dead worker from command routing', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.advance(3000)
    host.command()
    expect(worker.terminated).toBe(true)
    expect(host.errors()).toHaveLength(1)
    expect(worker.commands).toHaveLength(0)
  })

  it('activation errors settle once and remove command routing', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    worker.onmessage({ data: { type: 'error', error: 'Activation failed' } })
    host.advance(3000)
    host.command()
    expect(worker.terminated).toBe(true)
    expect(host.errors()).toHaveLength(1)
    expect(worker.commands).toHaveLength(0)
  })

  it('reset and replacement release each worker Blob URL', () => {
    const host = new SandboxHarness()
    host.load()
    host.load('second')
    expect(host.revoked).toEqual([host.urls[0]])
    host.send({ type: 'reset' })
    expect(host.revoked).toEqual(host.urls)
  })

  it('successful results cancel their command timers', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.ready(worker)
    host.command()
    worker.onmessage({ data: { type: 'result', requestId: 'request', result: {} } })
    host.advance(3000)
    expect(host.errors()).toEqual([])
    expect(worker.terminated).toBe(false)
  })

  it('recoverable command errors cancel their timers and allow retry', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.ready(worker)
    host.command()
    worker.onmessage({ data: { type: 'error', requestId: 'request', error: 'Command failed' } })
    host.advance(3000)
    expect(host.errors()).toHaveLength(1)
    expect(worker.terminated).toBe(false)
    expect(host.errors()[0].type).toBe('error')
    host.command('first', 'retry')
    expect(worker.commands).toHaveLength(2)
  })

  it('worker errors cancel all remaining timers', () => {
    const host = new SandboxHarness()
    const worker = host.load()
    host.ready(worker)
    host.command()
    worker.onerror()
    host.advance(3000)
    expect(host.errors()).toHaveLength(1)
    expect(worker.terminated).toBe(true)
  })

  it('constructor failure releases the URL and reports one activation failure', () => {
    const host = new SandboxHarness({ failConstruction: true })
    host.load()
    host.advance(3000)
    expect(host.errors()).toHaveLength(1)
    expect(host.revoked).toEqual(host.urls)
  })
})
