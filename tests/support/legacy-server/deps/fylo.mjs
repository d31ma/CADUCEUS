// Adapted from FYLO 26.33.02: clients/node/fylo.mjs.
// Drives the standalone `fylo` binary through its persistent NDJSON protocol.
// CADUCEUS adds its authenticated bridge transport and a large-file fallback.

import { spawn } from 'node:child_process'
import { createHash, createHmac, randomBytes } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { readFile } from 'node:fs/promises'
import { createConnection } from 'node:net'
import { readdir } from 'node:fs/promises'
import { isAbsolute, relative, resolve } from 'node:path'

const DEFAULT_MAX_REQUEST_BYTES = 1024 * 1024
const DEFAULT_MAX_RESPONSE_BYTES = 8 * 1024 * 1024
const MAX_FRAME_BYTES = 64 * 1024 * 1024
const MAX_SHARD_WIDTH = 4
const responseDecoder = new TextDecoder('utf-8', { fatal: true })
const ENVIRONMENT_NAME = /^[A-Za-z_][A-Za-z0-9_]*$/

function protocolError(code, message) {
  const error = /** @type {Error & { code: string }} */ (new Error(message))
  error.code = code
  return error
}

function frameLimit(value, fallback, name, minimum) {
  const limit = value ?? fallback
  if (!Number.isSafeInteger(limit) || limit < minimum || limit > MAX_FRAME_BYTES) {
    throw new RangeError(`${name} must be an integer between ${minimum} and ${MAX_FRAME_BYTES}`)
  }
  return limit
}

function environmentError(path, line, message) {
  return new SyntaxError(`${path}:${line}: ${message}`)
}

function quotedEnvironmentValue(path, line, input, quote) {
  let value = ''
  for (let index = 1; index < input.length; index += 1) {
    const character = input[index]
    if (character === quote) {
      const trailing = input.slice(index + 1).trim()
      if (trailing && !trailing.startsWith('#')) {
        throw environmentError(path, line, 'unexpected characters after quoted value')
      }
      return value
    }
    if (quote === '"' && character === '\\') {
      index += 1
      if (index >= input.length) {
        throw environmentError(path, line, 'unterminated escape sequence')
      }
      const escaped = input[index]
      value += escaped === 'n'
        ? '\n'
        : escaped === 'r'
          ? '\r'
          : escaped === 't'
            ? '\t'
            : escaped === '"' || escaped === '\\'
              ? escaped
              : `\\${escaped}`
      continue
    }
    value += character
  }
  throw environmentError(path, line, 'unterminated quoted value')
}

function parseEnvironmentFile(path) {
  let source
  try {
    source = readFileSync(path, 'utf8')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    throw new Error(`cannot read FYLO environment file ${path}: ${detail}`)
  }
  if (source.charCodeAt(0) === 0xfeff) source = source.slice(1)
  const values = Object.create(null)
  for (const [index, original] of source.split(/\r?\n/).entries()) {
    const line = index + 1
    let entry = original.trim()
    if (!entry || entry.startsWith('#')) continue
    if (/^export\s+/.test(entry)) entry = entry.replace(/^export\s+/, '')
    const separator = entry.indexOf('=')
    if (separator < 0) throw environmentError(path, line, 'expected NAME=VALUE')
    const name = entry.slice(0, separator).trim()
    if (!ENVIRONMENT_NAME.test(name)) {
      throw environmentError(path, line, `invalid environment variable name ${JSON.stringify(name)}`)
    }
    const input = entry.slice(separator + 1).trimStart()
    if (input.startsWith('"') || input.startsWith("'")) {
      values[name] = quotedEnvironmentValue(path, line, input, input[0])
      continue
    }
    const comment = input.indexOf('#')
    values[name] = (comment < 0 ? input : input.slice(0, comment)).trimEnd()
  }
  return values
}

function childEnvironment(config) {
  if (config === undefined) return undefined
  const overrides = typeof config === 'string' ? parseEnvironmentFile(config) : config
  if (!overrides || typeof overrides !== 'object' || Array.isArray(overrides)) {
    throw new TypeError('env must be a .env file path or an environment variable object')
  }
  const environment = Object.create(null)
  for (const [name, value] of Object.entries(process.env)) {
    environment[process.platform === 'win32' ? name.toUpperCase() : name] = value
  }
  for (const [name, value] of Object.entries(overrides)) {
    if (!name || name.includes('=') || name.includes('\0')) {
      throw new TypeError(`invalid environment variable name ${JSON.stringify(name)}`)
    }
    const environmentName = process.platform === 'win32' ? name.toUpperCase() : name
    if (value === undefined || value === null) {
      delete environment[environmentName]
    } else {
      const environmentValue = String(value)
      if (environmentValue.includes('\0')) {
        throw new TypeError(
          `invalid environment variable value for ${JSON.stringify(name)}: NUL is not allowed`,
        )
      }
      environment[environmentName] = environmentValue
    }
  }
  return environment
}

const RESERVED = new Set(['then'])
const FACADE = {
  get(target, prop, receiver) {
    if (typeof prop === 'symbol' || RESERVED.has(prop) || prop in target) {
      return Reflect.get(target, prop, receiver)
    }
    return target.collection(String(prop))
  },
}

export class Fylo {
  /**
   * @param {string} root
   * @param {{ binary?: string, env?: string | Record<string, string | number | boolean | null | undefined>, worm?: boolean, exclusiveRoot?: boolean, strictSchema?: boolean, shardWidth?: number, bridgeAddress?: string, bridgeSecret?: string, maxRequestBytes?: number, maxResponseBytes?: number }} [opts]
   */
  constructor(root, opts = {}) {
    this.root = resolve(root)
    this._binary = opts.binary ?? 'fylo'
    this._worm = Boolean(opts.worm)
    this._exclusiveRoot = opts.exclusiveRoot !== false
    this._strictSchema = Boolean(opts.strictSchema)
    this._shardWidth = opts.shardWidth
    this._env = opts.env
    this._bridgeAddress = opts.bridgeAddress
    this._bridgeSecret = opts.bridgeSecret
    this.maxRequestBytes = frameLimit(
      opts.maxRequestBytes,
      DEFAULT_MAX_REQUEST_BYTES,
      'maxRequestBytes',
      256,
    )
    this.maxResponseBytes = frameLimit(
      opts.maxResponseBytes,
      DEFAULT_MAX_RESPONSE_BYTES,
      'maxResponseBytes',
      1024,
    )
    this._queue = []
    this._responseBuffer = Buffer.allocUnsafe(this.maxResponseBytes)
    this._responseLength = 0
    this._responseOversized = false
    this._spawn()
    this.queue = Object.freeze({
      publish: (topic, payload, options = {}) => this.queuePublish(topic, payload, options),
      claim: (topic, group, options = {}) => this.queueClaim(topic, group, options),
      ack: (topic, group, delivery, receipt) => this.queueAck(
        topic, group, typeof delivery === 'string' ? delivery : delivery.id,
        receipt ?? delivery.receipt,
      ),
      nack: (topic, group, delivery, options = {}) => this.queueNack(
        topic, group, typeof delivery === 'string' ? delivery : delivery.id,
        typeof delivery === 'string' ? options.receipt : delivery.receipt, options,
      ),
      extend: (topic, group, delivery, receiptOrTimeout, visibilityTimeoutMs) => this.queueExtend(
        topic, group, typeof delivery === 'string' ? delivery : delivery.id,
        typeof delivery === 'string' ? receiptOrTimeout : delivery.receipt,
        typeof delivery === 'string' ? visibilityTimeoutMs : receiptOrTimeout,
      ),
      stats: (topic, group) => this.queueStats(topic, group),
      deadLetters: (topic, group, limit) => this.queueDeadLetters(topic, group, limit),
    })
    return new Proxy(this, FACADE)
  }

  _spawn() {
    if (this._bridgeAddress) {
      if (typeof this._bridgeSecret !== 'string' || Buffer.byteLength(this._bridgeSecret) < 32) {
        throw new Error('FYLO bridge secret must contain at least 32 bytes')
      }
      this.ready = this._sendOp('handshake', {}).then(identity => {
        this.identity = identity
        return identity
      })
      void this.ready.catch(() => {})
      return
    }
    const args = ['exec', '--loop', '--root', this.root]
    if (this._worm) args.push('--worm')
    if (this._exclusiveRoot) args.push('--exclusive-root')
    args.push('--max-request-bytes', String(this.maxRequestBytes))
    args.push('--max-response-bytes', String(this.maxResponseBytes))
    if (this._strictSchema) args.push('--strict-schema')
    if (this._shardWidth !== undefined) {
      if (
        !Number.isInteger(this._shardWidth)
        || this._shardWidth < 0
        || this._shardWidth > MAX_SHARD_WIDTH
      ) {
        throw new RangeError(`shardWidth must be an integer between 0 and ${MAX_SHARD_WIDTH}`)
      }
      args.push('--shard-width', String(this._shardWidth))
    }
    const process = spawn(this._binary, args, {
      stdio: ['pipe', 'pipe', 'inherit'],
      env: childEnvironment(this._env),
    })
    this._proc = process
    process.stdout.on('data', chunk => this._onData(chunk))
    process.on('exit', (code, signal) => {
      const reason = signal ? `signal ${signal}` : `code ${code ?? 'unknown'}`
      const error = new Error(`fylo process exited (${reason})`)
      for (const pending of this._queue.splice(0)) pending.reject(error)
    })
    process.on('error', error => {
      for (const pending of this._queue.splice(0)) pending.reject(error)
    })
    process.stdin.on('error', () => {})
    this.ready = this._sendOp('handshake', {})
      .then(identity => {
        if (
          identity?.machine?.maxRequestBytes !== this.maxRequestBytes
          || identity?.machine?.maxResponseBytes !== this.maxResponseBytes
        ) {
          process.kill()
          throw protocolError(
            'EPROTOCOL_LIMIT',
            'FYLO handshake frame limits do not match the client configuration',
          )
        }
        this.identity = identity
        return identity
      })
      .catch(error => {
        if (/Unsupported machine operation: handshake/i.test(error?.message ?? '')) {
          const identity = {
            runtimeVersion: '26.30.04-or-earlier',
            protocolVersion: 0,
            capabilities: { handshake: false, machineAccess: false },
          }
          this.identity = identity
          return identity
        }
        throw error
      })
    void this.ready.catch(() => {})
  }

  _currentProcess() {
    if (this._bridgeAddress) throw new Error('FYLO is owned by the Go bridge')
    const process = this._proc
    if (!process) throw new Error('fylo process has not started')
    return process
  }

  collection(name) {
    return {
      create: kind => this.createCollection(name, kind),
      drop: () => this.dropCollection(name),
      inspect: () => this.inspectCollection(name),
      rebuild: () => this.rebuildCollection(name),
      put: (data, meta) => this.putData(name, data, meta),
      putFile: (filePath, options) => this.putFile(name, filePath, options),
      bytes: id => this.getFileBytes(name, id),
      get: id => this.getDoc(name, id),
      getMeta: id => this.getMeta(name, id),
      setMeta: (id, meta) => this.setMeta(name, id, meta),
      latest: (id, onlyId) => this.getLatest(name, id, onlyId),
      patch: (id, newDoc, oldDoc) => this.patchDoc(name, id, newDoc, oldDoc),
      delete: id => this.delDoc(name, id),
      restore: id => this.restoreDoc(name, id),
      find: query => this.findDocs(name, query),
      findPage: (query, page) => this.findDocsPage(name, query, page),
    }
  }

  _onData(chunk) {
    const bytes = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)
    let start = 0
    while (start < bytes.byteLength) {
      const newline = bytes.indexOf(0x0a, start)
      const end = newline === -1 ? bytes.byteLength : newline
      this._appendResponse(bytes.subarray(start, end))
      if (newline === -1) return
      if (this._responseOversized) {
        this._abortProtocol(protocolError(
          'EFRAME_RESPONSE_TOO_LARGE',
          `FYLO response exceeds ${this.maxResponseBytes} bytes`,
        ))
        return
      }
      const pending = this._queue.shift()
      if (pending) {
        try {
          const text = responseDecoder.decode(
            this._responseBuffer.subarray(0, this._responseLength),
          )
          pending.resolve(JSON.parse(text))
        } catch {
          const error = protocolError(
            'EFRAME_RESPONSE_INVALID',
            'FYLO returned malformed UTF-8 or JSON',
          )
          pending.reject(error)
          this._abortProtocol(error)
          return
        }
      }
      this._responseLength = 0
      this._responseOversized = false
      start = newline + 1
    }
  }

  _appendResponse(bytes) {
    if (this._responseOversized || bytes.byteLength === 0) return
    if (bytes.byteLength > this.maxResponseBytes - this._responseLength) {
      this._responseLength = 0
      this._responseOversized = true
      return
    }
    this._responseBuffer.set(bytes, this._responseLength)
    this._responseLength += bytes.byteLength
  }

  _abortProtocol(error) {
    for (const pending of this._queue.splice(0)) pending.reject(error)
    this._currentProcess().kill()
  }

  request(op) {
    if (this._bridgeAddress) return this._bridgeRequest(op)
    return new Promise((resolve, reject) => {
      const process = this._currentProcess()
      if (process.exitCode !== null) return reject(new Error('fylo process exited'))
      let payload
      try {
        payload = Buffer.from(JSON.stringify(op), 'utf8')
      } catch (error) {
        reject(error)
        return
      }
      if (payload.byteLength > this.maxRequestBytes) {
        reject(protocolError(
          'EFRAME_REQUEST_TOO_LARGE',
          `FYLO request exceeds ${this.maxRequestBytes} bytes`,
        ))
        return
      }
      this._queue.push({ resolve, reject })
      process.stdin.write(Buffer.concat([payload, Buffer.from('\n')]))
    })
  }

  _bridgeRequest(op) {
    return new Promise((resolveRequest, rejectRequest) => {
      let payload
      try {
        payload = Buffer.from(JSON.stringify(op), 'utf8')
      } catch (error) {
        rejectRequest(error)
        return
      }
      if (payload.byteLength > this.maxRequestBytes) {
        rejectRequest(protocolError(
          'EFRAME_REQUEST_TOO_LARGE',
          `FYLO request exceeds ${this.maxRequestBytes} bytes`,
        ))
        return
      }
      const endpoint = parseBridgeAddress(this._bridgeAddress)
      if (!endpoint) {
        rejectRequest(new Error('FYLO_BRIDGE_ADDRESS must be an explicit loopback host and TCP port'))
        return
      }
      const { host, port } = endpoint
      const timestamp = Date.now()
      const nonce = randomBytes(24).toString('base64url')
      const encoded = payload.toString('base64url')
      const message = `${timestamp}.${nonce}.${encoded}`
      const bridgeSecret = this._bridgeSecret
      if (!bridgeSecret) {
        rejectRequest(new Error('FYLO bridge secret is unavailable'))
        return
      }
      const signature = createHmac('sha256', bridgeSecret).update(message).digest('hex')
      const frame = Buffer.from(`${JSON.stringify({
        version: 1,
        timestamp,
        nonce,
        payload: encoded,
        signature,
      })}\n`)
      const socket = createConnection({ host, port })
      let response = Buffer.alloc(0)
      let settled = false
      const finish = (error, value) => {
        if (settled) return
        settled = true
        socket.destroy()
        if (error) rejectRequest(error)
        else resolveRequest(value)
      }
      socket.setTimeout(10_000, () => finish(protocolError('ETIMEOUT', 'FYLO bridge timed out')))
      socket.once('error', error => finish(error))
      socket.on('data', chunk => {
        response = Buffer.concat([response, Buffer.from(chunk)])
        if (response.byteLength > this.maxResponseBytes + 1) {
          finish(protocolError(
            'EFRAME_RESPONSE_TOO_LARGE',
            `FYLO bridge response exceeds ${this.maxResponseBytes} bytes`,
          ))
          return
        }
        const newline = response.indexOf(0x0a)
        if (newline === -1) return
        try {
          finish(undefined, JSON.parse(response.subarray(0, newline).toString('utf8')))
        } catch {
          finish(protocolError('EFRAME_RESPONSE_INVALID', 'FYLO bridge returned malformed JSON'))
        }
      })
      socket.once('connect', () => socket.write(frame))
    })
  }

  async _sendOp(op, fields) {
    const payload = { op }
    for (const [key, value] of Object.entries(fields)) {
      if (value !== undefined) payload[key] = value
    }
    const response = await this.request(payload)
    if (!response.ok) {
      const error = /** @type {Error & { code?: string }} */ (
        new Error(response.error?.message ?? 'fylo error')
      )
      if (response.error?.name) error.name = response.error.name
      if (response.error?.code) error.code = response.error.code
      throw error
    }
    return response.result
  }

  async _op(op, fields) {
    await this.ready
    return await this._sendOp(op, fields)
  }

  handshake() {
    return this.ready
  }

  _accessOp(op, fields) {
    return this._accessTask(access => this._op(op, { ...fields, access }))
  }

  _accessTask(task) {
    let access
    let started = false
    const operation = Promise.resolve().then(() => {
      started = true
      return task(access)
    })
    return Object.assign(operation, {
      as(input) {
        if (started) throw new Error('as() must be called before the operation starts')
        access = input
        return operation
      },
    })
  }

  createCollection(collection, kind = 'document') {
    return this._op('createCollection', { collection, kind })
  }
  dropCollection(collection) {
    return this._op('dropCollection', { collection })
  }
  inspectCollection(collection) {
    return this._op('inspectCollection', { collection })
  }
  rebuildCollection(collection) {
    return this._op('rebuildCollection', { collection })
  }
  queuePublish(topic, payload, options = {}) {
    return this._op('queuePublish', {
      topic, payload, delayMs: options.delayMs, idempotencyKey: options.idempotencyKey,
    })
  }
  queueClaim(topic, group, options = {}) {
    return this._op('queueClaim', {
      topic, group, maxMessages: options.maxMessages,
      visibilityTimeoutMs: options.visibilityTimeoutMs, maxAttempts: options.maxAttempts,
    })
  }
  queueAck(topic, group, id, receipt) {
    return this._op('queueAck', { topic, group, id, receipt })
  }
  queueNack(topic, group, id, receipt, options = {}) {
    return this._op('queueNack', {
      topic, group, id, receipt, delayMs: options.delayMs, reason: options.reason,
    })
  }
  queueExtend(topic, group, id, receipt, visibilityTimeoutMs) {
    return this._op('queueExtend', { topic, group, id, receipt, visibilityTimeoutMs })
  }
  queueStats(topic, group) {
    return this._op('queueStats', { topic, group })
  }
  queueDeadLetters(topic, group, limit) {
    return this._op('queueDeadLetters', { topic, group, limit })
  }
  putData(collection, data, meta) {
    return this._accessOp('putData', { collection, data, meta })
  }
  putFile(collection, filePath, options = {}) {
    if (!isAbsolute(filePath)) throw new Error('Fylo bucket source path must be absolute')
    return this._accessOp('putData', {
      collection,
      file: { path: filePath, key: options.key },
      fileOptions: options.maxBytes === undefined ? undefined : { maxBytes: options.maxBytes },
      meta: options.meta,
    })
  }
  getFileBytes(collection, id) {
    return this._accessTask(async access => {
      try {
        const pending = this.getFileData(collection, id)
        const result = access ? await pending.as(access) : await pending
        if (!result || result.encoding !== 'base64' || typeof result.data !== 'string') {
          throw new Error(`Invalid Fylo bucket response for ${collection}/${id}`)
        }
        const bytes = Buffer.from(result.data, 'base64')
        if (bytes.byteLength !== result.contentLength) throw new Error('Fylo bucket length mismatch')
        const checksum = createHash('sha256').update(bytes).digest('hex')
        if (checksum !== result.checksumSHA256) throw new Error('Fylo bucket checksum mismatch')
        return new Uint8Array(bytes)
      } catch (error) {
        const code = /** @type {{ code?: string }} */ (error)?.code
        if (code !== 'EFRAME_RESPONSE_TOO_LARGE' && code !== 'EUNSUPPORTEDOP') throw error
      }

      // FYLO's supported inline output is bounded by the response frame. Large
      // legacy attachments retain the authorized local fallback until the Go
      // bridge owns a dedicated, path-confined getFileData output directory.
      if (!/^[A-Za-z0-9][A-Za-z0-9_-]*$/.test(collection)) throw new Error('Invalid Fylo bucket name')
      if (!/^[A-Za-z0-9_-]+$/.test(id)) throw new Error('Invalid Fylo bucket file ID')
      const pendingDocuments = this.getDoc(collection, id)
      const documents = access ? await pendingDocuments.as(access) : await pendingDocuments
      const name = documents?.[id]?.name
      if (typeof name !== 'string' || !name.startsWith(`${id}.`) || name.includes('/') || name.includes('\\')) {
        throw new Error(`Invalid Fylo bucket metadata for ${collection}/${id}`)
      }
      // The shard directory under docs/ is FYLO's own business. Discover it
      // rather than assuming a layout for the exceptional oversized read.
      const bucketRoot = resolve(this.root, '.buckets', collection)
      const docsRoot = resolve(bucketRoot, 'docs')
      const target = await findBucketObject(docsRoot, name)
      if (!target) throw new Error(`Fylo bucket object ${collection}/${name} is not present`)
      if (relative(bucketRoot, target).startsWith('..')) throw new Error('Fylo bucket path escapes its root')
      return new Uint8Array(await readFile(target))
    })
  }
  batchPutData(collection, batch) {
    return this._accessOp('batchPutData', { collection, batch })
  }
  getDoc(collection, id) {
    return this._accessOp('getDoc', { collection, id })
  }
  getFileData(collection, id, path) {
    return this._accessOp('getFileData', { collection, id, path })
  }
  getMeta(collection, id) {
    return this._accessOp('getMeta', { collection, id })
  }
  setMeta(collection, id, meta) {
    return this._accessOp('setMeta', { collection, id, meta })
  }
  getLatest(collection, id, onlyId = false) {
    return this._accessOp('getLatest', { collection, id, onlyId })
  }
  patchDoc(collection, id, newDoc, oldDoc) {
    return this._accessOp('patchDoc', { collection, id, newDoc, oldDoc })
  }
  patchDocs(collection, update) {
    return this._accessOp('patchDocs', { collection, update })
  }
  delDoc(collection, id) {
    return this._accessOp('delDoc', { collection, id })
  }
  delDocs(collection, criteria) {
    return this._accessOp('delDocs', { collection, delete: criteria })
  }
  restoreDoc(collection, id) {
    return this._accessOp('restoreDoc', { collection, id })
  }
  findDocs(collection, query) {
    return this._accessOp('findDocs', { collection, query })
  }
  findDeletedDocs(collection, query = {}) {
    return this._accessOp('findDeletedDocs', { collection, query })
  }
  findDocsPage(collection, query, page = {}) {
    return this._accessOp('findDocs', { collection, query, page })
  }
  findDeletedDocsPage(collection, query = {}, page = {}) {
    return this._accessOp('findDeletedDocs', { collection, query, page })
  }
  joinDocs(join) {
    return this._accessOp('joinDocs', { join })
  }
  executeSQL(sql, access) {
    return this._op('executeSQL', { sql, access })
  }
  sql(strings, ...values) {
    let statement = strings[0]
    for (let index = 0; index < values.length; index++) {
      statement += Fylo._sqlValue(values[index]) + strings[index + 1]
    }
    let access
    let started = false
    const operation = Promise.resolve().then(() => {
      started = true
      return this.executeSQL(statement, access)
    })
    return Object.assign(operation, {
      as(input) {
        if (started) throw new Error('as() must be called before SQL execution starts')
        access = input
        return operation
      },
    })
  }
  static _sqlValue(value) {
    if (value === null || value === undefined) return 'NULL'
    if (typeof value === 'number') {
      if (!Number.isFinite(value)) throw new Error('SQL value must be a finite number')
      return String(value)
    }
    if (typeof value === 'boolean') return value ? 'true' : 'false'
    if (typeof value === 'bigint') return value.toString()
    if (value instanceof Date) return `'${value.toISOString().replaceAll("'", "''")}'`
    if (typeof value === 'object') throw new Error('SQL values must be scalar')
    return `'${String(value).replaceAll("'", "''")}'`
  }
  importBulkData(collection, url, limitOrOptions) {
    return this._op('importBulkData', { collection, url, limitOrOptions })
  }
  close() {
    if (this._bridgeAddress) return Promise.resolve()
    return new Promise(/** @param {(value?: void) => void} resolve */ resolve => {
      const process = this._currentProcess()
      if (process.exitCode !== null) return resolve()
      process.once('exit', () => resolve())
      process.stdin.end()
    })
  }
}

function parseBridgeAddress(address) {
  const value = String(address)
  const match = value.match(/^\[([^\]]+)\]:(\d+)$/) ?? value.match(/^([^:]+):(\d+)$/)
  if (!match) return undefined
  const host = match[1]
  const port = Number(match[2])
  if (
    !['127.0.0.1', '::1'].includes(host)
    || !Number.isSafeInteger(port)
    || port < 1
    || port > 65535
  ) {
    return undefined
  }
  return { host, port }
}


/**
 * Find one bucket object by file name under a bucket's docs tree.
 *
 * The shard layout is FYLO's private detail and has already changed once
 * between releases, so callers locate the object instead of computing where it
 * ought to be.
 *
 * @param {string} docsRoot
 * @param {string} name
 * @returns {Promise<string | null>}
 */
async function findBucketObject(docsRoot, name) {
  let shards
  try {
    shards = await readdir(docsRoot, { withFileTypes: true })
  } catch {
    return null
  }
  for (const shard of shards) {
    if (!shard.isDirectory()) {
      if (shard.name === name) return resolve(docsRoot, name)
      continue
    }
    const shardPath = resolve(docsRoot, shard.name)
    let entries
    try {
      entries = await readdir(shardPath)
    } catch {
      continue
    }
    if (entries.includes(name)) return resolve(shardPath, name)
  }
  return null
}
