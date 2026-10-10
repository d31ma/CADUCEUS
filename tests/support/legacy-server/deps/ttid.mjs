// Vendored from TTID 26.32.03: clients/node/ttid.mjs.

import { spawn } from 'node:child_process'

export class TTID {
  /** @param {{ binary?: string }} [opts] */
  constructor(opts = {}) {
    this._proc = spawn(opts.binary ?? 'ttid', ['exec', '--loop'], {
      stdio: ['pipe', 'pipe', 'inherit'],
    })
    this._queue = []
    this._buffer = ''
    this._proc.stdout.setEncoding('utf8')
    this._proc.stdout.on('data', chunk => this._onData(chunk))
    this._proc.on('exit', () => {
      const error = new Error('ttid process exited')
      for (const pending of this._queue.splice(0)) pending.reject(error)
    })
    this._proc.on('error', error => {
      for (const pending of this._queue.splice(0)) pending.reject(error)
    })
    this._proc.stdin.on('error', () => {})
  }

  _onData(chunk) {
    this._buffer += chunk
    let newline
    while ((newline = this._buffer.indexOf('\n')) !== -1) {
      const line = this._buffer.slice(0, newline).trim()
      this._buffer = this._buffer.slice(newline + 1)
      if (!line) continue
      const pending = this._queue.shift()
      if (pending) pending.resolve(JSON.parse(line))
    }
  }

  request(op) {
    return new Promise((resolve, reject) => {
      if (this._proc.exitCode !== null) return reject(new Error('ttid process exited'))
      this._queue.push({ resolve, reject })
      this._proc.stdin.write(JSON.stringify(op) + '\n')
    })
  }

  async _op(op, fields) {
    const payload = { op }
    for (const [key, value] of Object.entries(fields)) {
      if (value !== undefined) payload[key] = value
    }
    const response = await this.request(payload)
    if (!response.ok) throw new Error(response.error?.message ?? 'ttid error')
    return response.result
  }

  generate(id, del) {
    return this._op('generate', { id, delete: del })
  }
  decodeTime(id) {
    return this._op('decodeTime', { id })
  }
  isTTID(id) {
    return this._op('isTTID', { id })
  }
  canonicalize(id) {
    return this._op('canonicalize', { id })
  }
  isUUID(id) {
    return this._op('isUUID', { id })
  }

  close() {
    return new Promise(resolve => {
      if (this._proc.exitCode !== null) return resolve()
      this._proc.on('exit', () => resolve())
      this._proc.stdin.end()
    })
  }
}
