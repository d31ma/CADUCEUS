class TacApplicationEvents {
constructor({ changed, failed, calling = (owner, work) => work(), announced = () => {} }) {
this.changed = changed
this.failed = failed
this.calling = calling
this.announced = announced
this.listeners = new Map()
this.retained = new Map()
this.owners = new WeakMap()
this.topics = new Map()
this.listenerCount = 0
this.pending = 0
this.depth = 0
this.publications = 0
this.resetTimer = null
}
snapshot(value) {
const active = new Set()
let nodes = 0
let characters = 0
const text = (value) => this.fitsSize(characters += value.length)
const visit = (item, depth) => {
this.fitsStructure(++nodes > 8192 || depth > 32)
if (typeof item === 'string') { text(item); return }
if (item === null || typeof item === 'boolean') return
if (typeof item === 'number' && Number.isFinite(item)) return
if (typeof item !== 'object' || active.has(item)) throw new TypeError('Application events require finite JSON values.')
const prototype = Object.getPrototypeOf(item)
if (!Array.isArray(item) && prototype !== Object.prototype && prototype !== null) throw new TypeError('Application events require plain JSON values.')
const array = Array.isArray(item)
this.fitsStructure(array && item.length > 8192)
active.add(item)
let properties = 0
for (const key of Reflect.ownKeys(item)) {
if (array && key === 'length') continue
const descriptor = Object.getOwnPropertyDescriptor(item, key)
if (typeof key !== 'string' || !descriptor.enumerable || !Object.hasOwn(descriptor, 'value')) throw new TypeError('Application events require JSON data properties.')
if (array && (!/^(0|[1-9][0-9]*)$/.test(key) || Number(key) >= item.length)) throw new TypeError('Application event arrays require indexed JSON values.')
text(key)
properties += 1
visit(descriptor.value, depth + 1)
}
if (array && properties !== item.length) throw new TypeError('Application event arrays cannot contain holes.')
active.delete(item)
}
visit(value, 0)
const encoded = JSON.stringify(value)
this.fitsSize(new TextEncoder().encode(encoded).byteLength)
return JSON.parse(encoded)
}
fitsSize(size) {
if (size > 65536) throw new RangeError('Application event exceeds 64 KiB.')
}
fitsStructure(exceeded) {
if (exceeded) throw new RangeError('Application event structure exceeds its limit.')
}
name(value) {
if (typeof value !== 'string' || !/^[A-Za-z0-9._:-]{1,128}$/.test(value)) throw new TypeError('Invalid application event name.')
return value
}
publish(name, value, { retain = false } = {}) {
this.name(name)
if (this.depth >= 32 || this.publications >= 1024) throw new RangeError('Application event delivery cycle exceeds its limit.')
const snapshot = this.snapshot(value)
this.publications += 1
if (this.resetTimer === null) this.resetTimer = setTimeout(() => { this.publications = 0; this.resetTimer = null }, 0)
if (retain) {
if (!this.retained.has(name) && this.retained.size >= 128) throw new RangeError('Application event retention limit reached.')
this.retained.set(name, snapshot)
}
this.depth += 1
try {
for (const listener of [...(this.listeners.get(name) || [])]) this.deliver(listener, snapshot)
} finally { this.depth -= 1 }
}
send(owner, name, value) {
if (owner === null) return this.publish(name, value)
this.name(name)
const snapshot = this.snapshot(value)
for (const listener of this.owners.get(owner)?.listeners || []) {
if (listener.event === name) this.deliver(listener, snapshot)
}
}
async settle(value, state) {
if (this.pending >= 128) {
void Promise.resolve(value).catch(() => {})
throw new RangeError('Application event async admission limit reached.')
}
this.pending += 1
let timer
let cancel
try {
await Promise.race([
value,
new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Application event work timed out.')), 10000) }),
new Promise((_, reject) => {
cancel = () => reject(new Error('Application owner disposed.'))
if (state.controller.signal.aborted) cancel()
else state.controller.signal.addEventListener('abort', cancel, { once: true })
}),
])
} finally {
clearTimeout(timer)
state.controller.signal.removeEventListener('abort', cancel)
this.pending -= 1
}
}
deliver(listener, value) {
const { owner, state, method, condition, asynchronous, event, models, returns } = listener
if (state.disposed) return
if (this.pending >= 128 || (models && !this.fits(models, event, value))) { this.failed(); return }
if (condition && !this.matches(condition, value, event)) return
const complete = (result) => {
if (state.disposed) return
if (!asynchronous && returns) this.republish(result, state, returns)
this.changed()
}
const invoke = () => {
if (state.disposed) return
try {
const result = owner[method](this.snapshot(value))
if (!result || typeof result.then !== 'function') { complete(result); return }
void this.settle(result, state).then(complete, () => { if (!state.disposed) this.failed() })
} catch { this.failed() }
}
if (asynchronous) queueMicrotask(invoke)
else invoke()
}
attach(owner, metadata, resources) {
let state = this.owners.get(owner)
if (state) { state.resources = resources; return }
state = { owner, controller: new AbortController(), resources, metadata, listeners: [], injected: new Map(), disposed: false, initialized: false }
this.owners.set(owner, state)
try {
for (const { field, name } of metadata.resources || []) {
if (!Object.hasOwn(resources, name)) throw new TypeError('Application resource is unavailable.')
Object.defineProperty(owner, field, { configurable: true, enumerable: false, get: () => this.resource(state, name) })
}
for (const listener of metadata.listeners || []) this.listen(owner, state, listener)
} catch (error) { this.dispose(owner); throw error }
}
resource(state, name) {
if (name === 'lifecycle') return state.controller.signal
if (name === 'view') return state.resources.view
if (!state.injected.has(name)) state.injected.set(name, Object.freeze(this.injectable(state, name)))
return state.injected.get(name)
}
injectable(state, name) {
if (name === 'applicationEventPublisher') return { publishEvent: (event, value, retain = false) => {
this.alive(state)
this.publish(event, value, { retain: retain === true })
this.announced(state.resources.view, event, value)
} }
return Object.fromEntries(Object.keys(state.resources[name]).map(method => [method, (...args) => {
this.alive(state)
return this.calling(state.owner, () => state.resources[name][method](...args))
}]))
}
alive(state) {
if (state.disposed) throw new Error('Application owner disposed.')
}
listen(owner, state, listener) {
if (typeof owner[listener.method] !== 'function') throw new TypeError('Application listener method is unavailable.')
for (const name of listener.events || [listener.event]) {
this.name(name)
if (this.listenerCount >= 1024 || (!this.listeners.has(name) && this.listeners.size >= 128)) throw new RangeError('Application listener limit reached.')
const entry = { order: 0, ...listener, owner, state, event: name, sequence: this.listenerCount }
const listeners = this.listeners.get(name) || new Set()
listeners.add(entry)
this.listeners.set(name, new Set([...listeners].sort((left, right) => left.order - right.order || left.sequence - right.sequence)))
this.listenerCount += 1
state.listeners.push(entry)
}
}
matches(matcher, value, event) {
const operand = (node) => {
if (node.k === 'lit') return node.v
if (node.k === 'event') return event
let current = value
for (const segment of node.path || []) {
if (current === null || typeof current !== 'object') return undefined
current = current[segment]
}
return current
}
const test = (node) => {
switch (node.k) {
case 'and': return test(node.l) && test(node.r)
case 'or': return test(node.l) || test(node.r)
case 'not': return !test(node.e)
case 'truthy': return Boolean(operand(node.e))
case 'cmp': {
const left = operand(node.l), right = operand(node.r)
if (node.op === 'eq') return left === right
if (node.op === 'ne') return left !== right
if (typeof left !== typeof right || (typeof left !== 'number' && typeof left !== 'string')) return false
if (node.op === 'lt') return left < right
if (node.op === 'le') return left <= right
if (node.op === 'gt') return left > right
return left >= right
}
default: return false
}
}
try { return test(matcher) } catch { return false }
}
republish(result, state, name) {
for (const item of [result].flat().slice(0, 32)) {
if (state.disposed) return
if (item != null) try { this.publish(name, item) } catch { this.failed() }
}
}
fits(models, shape, value) {
const next = (item, type) => this.fits(models, type, item), model = models[shape]
if (shape[0] === '?') return value === null || next(value, shape.slice(1))
if (shape[0] === '[') return Array.isArray(value) && value.every((item) => next(item, shape.slice(2)))
if (!model) return shape === 'any' || (shape === 'integer' ? Number.isInteger(value) : typeof value === shape)
const { open, fields } = model, keys = Object.keys(fields)
return value?.constructor === Object && (open || Object.keys(value).every((key) => keys.includes(key))) &&
keys.every((key) => Object.hasOwn(value, key) ? next(value[key], fields[key].replace('!', '')) : fields[key][0] !== '!')
}
async initialize(owner) {
const state = this.owners.get(owner)
if (!state || state.initialized || state.disposed) return
state.initialized = true
for (const listener of state.listeners) {
if (listener.replay && this.retained.has(listener.event)) this.deliver(listener, this.retained.get(listener.event))
}
for (const stream of state.metadata.streams || []) stream.topic ? this.topic(state, stream) : this.stream(owner, state, stream)
const method = state.metadata.post_construct
if (method) await this.settle(Promise.resolve().then(() => {
if (!state.disposed) return owner[method]()
}), state)
}
topic(state, stream) {
let shared = this.topics.get(stream.path)
if (!shared) {
shared = { owners: 0, disposed: false, controller: new AbortController() }
this.topics.set(stream.path, shared)
this.stream(null, shared, stream)
}
shared.owners++
state.controller.signal.addEventListener('abort', () => {
if (--shared.owners) return
shared.disposed = true
shared.controller.abort()
this.topics.delete(stream.path)
}, { once: true })
}
stream(owner, state, stream) {
const socket = stream.socket, Kind = globalThis[socket ? 'WebSocket' : 'EventSource']
if (typeof Kind !== 'function') { this.failed(); return }
const entry = { ...stream, owner, state }, waiting = []
let source = null, timer, tries = 0, last = null
const close = () => { clearTimeout(timer); source?.close(); source = null }
const wait = () => { clearTimeout(timer); if (!socket) timer = setTimeout(retry, 60000) }
const retry = () => {
close()
if (!state.disposed && tries < 5) timer = setTimeout(open, 1000 * 2 ** tries++)
}
const receive = ({ data, lastEventId }) => {
tries = 0
wait()
if (lastEventId) last = Number(lastEventId)
try {
const message = this.snapshot(JSON.parse(data))
if (!stream.topic) this.deliver(entry, message)
if (!stream.models || this.fits(stream.models, stream.event, message)) this.publish(stream.event, message)
} catch { this.failed() }
}
const open = () => {
if (state.disposed) return
if (socket) {
const url = new URL(stream.path, location.href)
url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
source = new Kind(url)
source.onopen = () => { for (const text of waiting.splice(0)) source.send(text) }
source.onclose = ({ code }) => { source = null; if (![1000, 1003, 1007, 1009].includes(code)) retry() }
} else {
source = new Kind(stream.topic && last !== null ? `${stream.path}?position=${last + 1}` : stream.path)
source.onerror = retry
source.addEventListener('end', close)
}
source.onmessage = receive
wait()
}
if (socket) Object.defineProperty(owner, 'send', { configurable: true, value: async (message) => {
this.alive(state)
const text = JSON.stringify(this.snapshot(message))
if (source?.readyState === 1) return source.send(text)
if (waiting.length >= 16) throw new RangeError('The socket is not open.')
waiting.push(text)
if (!source) { tries = 0; clearTimeout(timer); open() }
} })
state.controller.signal.addEventListener('abort', close, { once: true })
open()
}
dispose(owner) {
const state = this.owners.get(owner)
if (!state || state.disposed) return
state.disposed = true
state.controller.abort()
for (const listener of state.listeners) {
const listeners = this.listeners.get(listener.event)
if (listeners?.delete(listener)) this.listenerCount -= 1
if (!listeners?.size) this.listeners.delete(listener.event)
}
state.listeners.length = 0
const method = state.metadata.pre_destroy
if (method && state.initialized) {
try {
const result = owner[method]()
if (result && typeof result.then === 'function') {
void this.settle(result, { controller: new AbortController() }).catch(() => this.failed())
}
} catch { this.failed() }
}
}
}
class TacStorageError extends Error {
name = 'TacStorageError'
}
class TacCacheStorage {
static namePrefix = 'tachyon-cache:'
static pathPrefix = '/.tachyon/cache/'
static expiresHeader = 'x-tachyon-expires-at'
static usedHeader = 'x-tachyon-used-at'
static maxPreloadEntries = 10000
static memory = new Map()
static adapters = new Map()
static pending = new Map()
static draining = null
static loading = null
static noticed = false
static available() {
try { return globalThis.isSecureContext === true && typeof globalThis.caches?.open === 'function' } catch { return false }
}
static notice() {
if (TacCacheStorage.noticed) return
TacCacheStorage.noticed = true
console.info('Tachyon keeps Cached values in memory only on this page, because the browser Cache API needs HTTPS or localhost.')
}
static preload() {
TacCacheStorage.loading ??= TacCacheStorage.load().catch(() => {})
return TacCacheStorage.loading
}
static async load() {
if (!TacCacheStorage.available()) { TacCacheStorage.notice(); return }
for (const cacheName of await caches.keys()) {
if (!cacheName.startsWith(TacCacheStorage.namePrefix)) continue
const name = cacheName.slice(TacCacheStorage.namePrefix.length)
const cache = await caches.open(cacheName)
const found = []
for (const request of (await cache.keys()).slice(0, TacCacheStorage.maxPreloadEntries)) {
const entry = await TacCacheStorage.readResponse(name, request, await cache.match(request))
if (entry) found.push(entry)
else void cache.delete(request).catch(() => {})
}
found.sort((left, right) => left.usedAt - right.usedAt)
const entries = TacCacheStorage.entries(name)
for (const entry of found) if (!entries.has(entry.key)) entries.set(entry.key, { value: entry.value, expiresAt: entry.expiresAt })
}
}
static async readResponse(name, request, response) {
const prefix = `${TacCacheStorage.pathPrefix}${encodeURIComponent(name)}/`
const path = new URL(request.url).pathname
const expiresAt = Number(response?.headers.get(TacCacheStorage.expiresHeader))
if (!response || !path.startsWith(prefix) || !Number.isFinite(expiresAt) || expiresAt <= Date.now()) return null
try {
return {
key: decodeURIComponent(path.slice(prefix.length)),
value: JSON.parse(await response.text()),
expiresAt,
usedAt: Number(response.headers.get(TacCacheStorage.usedHeader)) || 0,
}
} catch { return null }
}
static entries(name) {
let entries = TacCacheStorage.memory.get(name)
if (!entries) { entries = new Map(); TacCacheStorage.memory.set(name, entries) }
return entries
}
static adapter(name) {
let adapter = TacCacheStorage.adapters.get(name)
if (adapter) return adapter
adapter = Object.freeze({
get: (key) => TacCacheStorage.read(name, key),
set: (key, value, execution) => TacCacheStorage.write(name, key, value, execution),
delete: (key) => TacCacheStorage.remove(name, key),
})
TacCacheStorage.adapters.set(name, adapter)
return adapter
}
static read(name, key) {
const entries = TacCacheStorage.entries(name)
const entry = entries.get(key)
if (!entry) return { hit: false }
if (entry.expiresAt <= Date.now()) { TacCacheStorage.remove(name, key); return { hit: false } }
entries.delete(key)
entries.set(key, entry)
return { hit: true, value: entry.value, expiresAt: entry.expiresAt }
}
static write(name, key, value, execution) {
const entries = TacCacheStorage.entries(name)
const entry = { value, expiresAt: execution.expiresAt }
entries.delete(key)
entries.set(key, entry)
TacCacheStorage.schedule(name, key, entry)
while (entries.size > execution.maxEntries) {
const [oldest] = entries.keys()
TacCacheStorage.remove(name, oldest)
}
}
static remove(name, key) {
TacCacheStorage.entries(name).delete(key)
TacCacheStorage.schedule(name, key, null)
}
static schedule(name, key, entry) {
if (!TacCacheStorage.available()) return
const id = JSON.stringify([name, key])
TacCacheStorage.pending.delete(id)
TacCacheStorage.pending.set(id, { name, key, entry })
TacCacheStorage.draining ??= TacCacheStorage.drain()
}
static async drain() {
try {
await TacCacheStorage.preload()
while (TacCacheStorage.pending.size) {
const [id, change] = TacCacheStorage.pending.entries().next().value
TacCacheStorage.pending.delete(id)
try { await TacCacheStorage.persist(change) } catch {}
}
} finally { TacCacheStorage.draining = null }
}
static async persist({ name, key, entry }) {
const cache = await caches.open(`${TacCacheStorage.namePrefix}${name}`)
const url = new URL(`${TacCacheStorage.pathPrefix}${encodeURIComponent(name)}/${encodeURIComponent(key)}`, location.origin)
if (!entry) { await cache.delete(url); return }
await cache.put(url, new Response(JSON.stringify(entry.value), {
headers: {
'content-type': 'application/json',
[TacCacheStorage.expiresHeader]: String(entry.expiresAt),
[TacCacheStorage.usedHeader]: String(Date.now()),
},
}))
}
static settled() {
return TacCacheStorage.draining ?? Promise.resolve()
}
}
class TacEntityStorage {
static databaseName = 'tachyon-store'
static scanLimit = 10000
static maxTransactions = 64
static wanted = new Map()
static current = null
static checked = ''
static queue = Promise.resolve()
static transactions = new Map()
static counter = 0
static adapter = Object.freeze({ execute: (command) => TacEntityStorage.execute(command) })
static declare(table, id, fields) {
const entry = TacEntityStorage.wanted.get(table) ?? { id, indexes: new Set() }
if (entry.id !== id) {
throw new TacStorageError(`Two entities keep rows in the IndexedDB store '${table}' but use different id fields ('${entry.id}' and '${id}'). Give one of them its own Table.`)
}
for (const field of fields) if (field !== id) entry.indexes.add(field)
TacEntityStorage.wanted.set(table, entry)
}
static schemaKey() {
return JSON.stringify([...TacEntityStorage.wanted].map(([table, entry]) => [table, entry.id, [...entry.indexes].sort()]).sort())
}
static database() {
const next = TacEntityStorage.queue.then(() => TacEntityStorage.ensure())
TacEntityStorage.queue = next.catch(() => undefined)
return next
}
static async ensure() {
const key = TacEntityStorage.schemaKey()
if (TacEntityStorage.current && TacEntityStorage.checked === key) return TacEntityStorage.current
if (typeof globalThis.indexedDB?.open !== 'function') throw new TacStorageError('This browser has no IndexedDB, so an entity with the "indexeddb" adapter cannot be stored here.')
let database = TacEntityStorage.current ?? TacEntityStorage.watch(await TacEntityStorage.open())
if (TacEntityStorage.missing(database)) {
const version = database.version + 1
database.close()
database = TacEntityStorage.watch(await TacEntityStorage.open(version))
}
TacEntityStorage.current = database
TacEntityStorage.checked = key
return database
}
static open(version) {
return new Promise((resolve, reject) => {
const request = version === undefined
? indexedDB.open(TacEntityStorage.databaseName)
: indexedDB.open(TacEntityStorage.databaseName, version)
request.onupgradeneeded = () => {
try { TacEntityStorage.upgrade(request.result, request.transaction) } catch (error) { request.transaction.abort(); reject(error) }
}
request.onsuccess = () => resolve(request.result)
request.onerror = () => reject(request.error)
})
}
static watch(database) {
database.onversionchange = () => {
database.close()
if (TacEntityStorage.current === database) TacEntityStorage.current = null
}
return database
}
static missing(database) {
for (const [table, entry] of TacEntityStorage.wanted) {
if (!database.objectStoreNames.contains(table)) return true
const store = database.transaction(table, 'readonly').objectStore(table)
TacEntityStorage.checkKey(table, store.keyPath, entry.id)
for (const field of entry.indexes) if (!store.indexNames.contains(field)) return true
}
return false
}
static upgrade(database, transaction) {
for (const [table, entry] of TacEntityStorage.wanted) {
const store = database.objectStoreNames.contains(table)
? transaction.objectStore(table)
: database.createObjectStore(table, { keyPath: entry.id })
TacEntityStorage.checkKey(table, store.keyPath, entry.id)
for (const field of entry.indexes) if (!store.indexNames.contains(field)) store.createIndex(field, field)
}
}
static checkKey(table, stored, wanted) {
if (stored === wanted) return
throw new TacStorageError(`The IndexedDB store '${table}' keeps its rows by '${stored}', but the entity now uses '${wanted}' as its Id. Tachyon never deletes stored rows, so it stops here. Keep the old Id field, or give the entity a new Table name.`)
}
static finished(transaction) {
return new Promise((resolve, reject) => {
transaction.oncomplete = () => resolve()
transaction.onerror = () => reject(transaction.error)
transaction.onabort = () => reject(transaction.error ?? new Error('The IndexedDB transaction was aborted.'))
})
}
static answer(request) {
return new Promise((resolve, reject) => {
request.onsuccess = () => resolve(request.result)
request.onerror = () => reject(request.error)
})
}
static async execute(command) {
if (command.operation === 'transaction') return TacEntityStorage.transaction(command)
TacEntityStorage.declare(command.table, command.idField, [])
const pending = command.transaction === undefined ? null : TacEntityStorage.transactions.get(command.transaction)
if (command.transaction !== undefined && !pending) throw new TacStorageError('The IndexedDB transaction is not open.')
if (command.operation === 'save') return TacEntityStorage.save(command, pending)
if (command.operation === 'delete') return TacEntityStorage.delete(command, pending)
if (command.operation === 'find') {
const rows = await TacEntityStorage.rows(command, [], pending)
return TacEntityStorage.page(rows, command.page, [], command.idField)
}
if (command.operation === 'query') return TacEntityStorage.query(command, pending)
throw new TacStorageError(`The IndexedDB adapter does not know the operation '${command.operation}'.`)
}
static transaction(command) {
if (command.action === 'begin') {
if (TacEntityStorage.transactions.size >= TacEntityStorage.maxTransactions) throw new TacStorageError('Too many IndexedDB transactions are open at once.')
const id = `indexeddb-${++TacEntityStorage.counter}`
TacEntityStorage.transactions.set(id, new Map())
return { transaction: id }
}
const writes = TacEntityStorage.transactions.get(command.transaction)
TacEntityStorage.transactions.delete(command.transaction)
if (!writes) throw new TacStorageError('The IndexedDB transaction is not open.')
if (command.action === 'commit') return TacEntityStorage.commit(writes).then(() => { TacEntityMirror.apply(writes); return null })
return null
}
static async commit(writes) {
if (!writes.size) return null
const database = await TacEntityStorage.database()
const transaction = database.transaction([...writes.keys()], 'readwrite')
const done = TacEntityStorage.finished(transaction)
for (const [table, rows] of writes) {
const store = transaction.objectStore(table)
for (const [id, row] of rows) {
if (row === null) store.delete(id)
else store.put(row)
}
}
await done
return null
}
static staged(pending, table) {
let rows = pending.get(table)
if (!rows) { rows = new Map(); pending.set(table, rows) }
return rows
}
static async save(command, pending) {
if (pending) { TacEntityStorage.staged(pending, command.table).set(command.value[command.idField], command.value); return command.value }
const database = await TacEntityStorage.database()
const transaction = database.transaction(command.table, 'readwrite')
const done = TacEntityStorage.finished(transaction)
transaction.objectStore(command.table).put(command.value)
await done
TacEntityMirror.apply(new Map([[command.table, new Map([[command.value[command.idField], command.value]])]]))
return command.value
}
static async delete(command, pending) {
if (pending) { TacEntityStorage.staged(pending, command.table).set(command.id, null); return true }
const database = await TacEntityStorage.database()
const transaction = database.transaction(command.table, 'readwrite')
const done = TacEntityStorage.finished(transaction)
transaction.objectStore(command.table).delete(command.id)
await done
TacEntityMirror.apply(new Map([[command.table, new Map([[command.id, null]])]]))
return true
}
static async rows(command, where, pending) {
const database = await TacEntityStorage.database()
const store = database.transaction(command.table, 'readonly').objectStore(command.table)
const indexed = where.find((predicate) => TacEntityStorage.usesIndex(store, predicate))
const source = indexed ? store.index(indexed.field) : store
const range = indexed ? TacEntityStorage.range(indexed) : null
const found = await TacEntityStorage.answer(source.getAll(range, TacEntityStorage.scanLimit + 1))
if (found.length > TacEntityStorage.scanLimit) {
throw new TacStorageError(`The IndexedDB store '${command.table}' has more than ${TacEntityStorage.scanLimit} rows to read for this query. Add a Store query that compares a string or number field, so Tachyon can use that field's index.`)
}
const staged = pending?.get(command.table)
if (!staged) return found
const merged = new Map(found.map((row) => [row[command.idField], row]))
for (const [id, row] of staged) {
if (row === null) merged.delete(id)
else merged.set(id, row)
}
return [...merged.values()]
}
static usesIndex(store, predicate) {
const key = (value) => (typeof value === 'number' && Number.isFinite(value)) || typeof value === 'string'
return ['eq', 'lt', 'lte', 'gt', 'gte'].includes(predicate.operator) && key(predicate.value) && store.indexNames.contains(predicate.field)
}
static range(predicate) {
const value = predicate.value
if (predicate.operator === 'eq') return IDBKeyRange.only(value)
if (predicate.operator === 'lt') return IDBKeyRange.upperBound(value, true)
if (predicate.operator === 'lte') return IDBKeyRange.upperBound(value)
if (predicate.operator === 'gt') return IDBKeyRange.lowerBound(value, true)
return IDBKeyRange.lowerBound(value)
}
static matches(row, predicate) {
const value = row[predicate.field]
const wanted = predicate.value
const comparable = (typeof value === 'number' && typeof wanted === 'number') || (typeof value === 'string' && typeof wanted === 'string')
switch (predicate.operator) {
case 'eq': return value === wanted || (wanted === null && value === undefined)
case 'ne': return !(value === wanted || (wanted === null && value === undefined))
case 'lt': return comparable && value < wanted
case 'lte': return comparable && value <= wanted
case 'gt': return comparable && value > wanted
case 'gte': return comparable && value >= wanted
case 'in': return wanted.includes(value)
case 'isNull': return value === null || value === undefined
case 'isNotNull': return value !== null && value !== undefined
default: return false
}
}
static compare(left, right) {
const rank = (value) => value === null || value === undefined ? 0 : typeof value === 'boolean' ? 1 : typeof value === 'number' ? 2 : typeof value === 'string' ? 3 : 4
if (rank(left) !== rank(right)) return rank(left) - rank(right)
if (rank(left) === 4) return JSON.stringify(left) < JSON.stringify(right) ? -1 : JSON.stringify(left) > JSON.stringify(right) ? 1 : 0
return left < right ? -1 : left > right ? 1 : 0
}
static page(rows, page, orderBy, idField) {
const order = page.sort.length
? page.sort.map((entry) => { const [field, direction] = entry.split(':'); return { field, direction } })
: orderBy
const ordering = order.some((entry) => entry.field === idField) ? order : [...order, { field: idField, direction: 'asc' }]
const sorted = rows.slice().sort((left, right) => {
for (const { field, direction } of ordering) {
const difference = TacEntityStorage.compare(left[field], right[field])
if (difference) return direction === 'desc' ? -difference : difference
}
return 0
})
return sorted.slice(page.offset, page.offset + page.size)
}
static async query(command, pending) {
const query = command.query
const rows = (await TacEntityStorage.rows(command, query.where, pending))
.filter((row) => query.where.every((predicate) => TacEntityStorage.matches(row, predicate)))
if (query.kind === 'count') return rows.length
if (query.kind === 'exists') return rows.length > 0
if (query.kind === 'delete') return TacEntityStorage.deleteRows(command, rows.map((row) => row[command.idField]), pending)
return TacEntityStorage.page(rows, query.page, query.orderBy, command.idField)
}
static async deleteRows(command, ids, pending) {
if (pending) { for (const id of ids) TacEntityStorage.staged(pending, command.table).set(id, null); return ids.length }
if (!ids.length) return 0
const database = await TacEntityStorage.database()
const transaction = database.transaction(command.table, 'readwrite')
const done = TacEntityStorage.finished(transaction)
const store = transaction.objectStore(command.table)
for (const id of ids) store.delete(id)
await done
TacEntityMirror.apply(new Map([[command.table, new Map(ids.map((id) => [id, null]))]]))
return ids.length
}
}
class TacEntityMirror {
static tables = new Map()
static keys = new Map()
static oversized = new Set()
static transactions = new Map()
static counter = 0
static loaded = false
static loading = null
static writing = Promise.resolve()
static preload() {
TacEntityMirror.loading ??= TacEntityMirror.load().catch((error) => { console.error(error) })
return TacEntityMirror.loading
}
static async load() {
try { if (typeof globalThis.indexedDB?.open !== 'function') return } catch { return }
if (!(await TacEntityMirror.databaseExists())) { TacEntityMirror.loaded = true; return }
const database = await TacEntityStorage.database()
for (const table of database.objectStoreNames) {
const store = database.transaction(table, 'readonly').objectStore(table)
const rows = await TacEntityStorage.answer(store.getAll(null, TacEntityStorage.scanLimit + 1))
TacEntityMirror.keys.set(table, store.keyPath)
if (rows.length > TacEntityStorage.scanLimit) { TacEntityMirror.oversized.add(table); continue }
TacEntityMirror.tables.set(table, new Map(rows.map((row) => [row[store.keyPath], row])))
}
TacEntityMirror.loaded = true
}
static async databaseExists() {
if (typeof indexedDB.databases !== 'function') return true
const databases = await indexedDB.databases()
return databases.some((database) => database.name === TacEntityStorage.databaseName)
}
static execute(command) {
if (command.operation === 'transaction') return TacEntityMirror.transaction(command)
const rows = TacEntityMirror.rows(command)
const pending = command.transaction == null ? null : TacEntityMirror.transactions.get(command.transaction)
if (command.transaction != null && !pending) throw new TacStorageError('The IndexedDB transaction is not open.')
if (command.operation === 'save') return TacEntityMirror.write(command, pending, command.value[command.idField], command.value) ?? command.value
if (command.operation === 'delete') return TacEntityMirror.write(command, pending, command.id, null) ?? true
const visible = TacEntityMirror.visible(rows, pending?.get(command.table))
if (command.operation === 'find') return TacEntityStorage.page(visible, command.page, [], command.idField)
if (command.operation !== 'query') throw new TacStorageError(`The IndexedDB adapter does not know the operation '${command.operation}'.`)
const query = command.query
const found = visible.filter((row) => query.where.every((predicate) => TacEntityStorage.matches(row, predicate)))
if (query.kind === 'count') return found.length
if (query.kind === 'exists') return found.length > 0
if (query.kind === 'delete') { for (const row of found) TacEntityMirror.write(command, pending, row[command.idField], null); return found.length }
return TacEntityStorage.page(found, query.page, query.orderBy, command.idField)
}
static rows(command) {
const table = command.table
if (!TacEntityMirror.loaded) throw new TacStorageError('Tachyon could not load the IndexedDB rows for this page, so this service cannot read its Store.')
TacEntityStorage.checkKey(table, TacEntityMirror.keys.get(table) ?? command.idField, command.idField)
const rows = TacEntityMirror.tables.get(table) ?? new Map()
if (TacEntityMirror.oversized.has(table) || rows.size > TacEntityStorage.scanLimit) {
throw new TacStorageError(`The IndexedDB store '${table}' has more than ${TacEntityStorage.scanLimit} rows. A WebAssembly or native Tac service reads its Store from memory, so keep this Table below that size or move the Store to a JavaScript or TypeScript service.`)
}
TacEntityStorage.declare(table, command.idField, [])
TacEntityMirror.tables.set(table, rows)
TacEntityMirror.keys.set(table, command.idField)
return rows
}
static write(command, pending, id, row) {
if (pending) { TacEntityStorage.staged(pending, command.table).set(id, row); return undefined }
const writes = new Map([[command.table, new Map([[id, row]])]])
TacEntityMirror.apply(writes)
TacEntityMirror.persist(writes)
return undefined
}
static visible(rows, staged) {
if (!staged) return [...rows.values()]
const merged = new Map(rows)
for (const [id, row] of staged) {
if (row === null) merged.delete(id)
else merged.set(id, row)
}
return [...merged.values()]
}
static transaction(command) {
if (command.action === 'begin') {
if (TacEntityMirror.transactions.size >= TacEntityStorage.maxTransactions) throw new TacStorageError('Too many IndexedDB transactions are open at once.')
const id = `indexeddb-memory-${++TacEntityMirror.counter}`
TacEntityMirror.transactions.set(id, new Map())
return { transaction: id }
}
const writes = TacEntityMirror.transactions.get(command.transaction)
TacEntityMirror.transactions.delete(command.transaction)
if (!writes) throw new TacStorageError('The IndexedDB transaction is not open.')
if (command.action === 'commit' && writes.size) {
TacEntityMirror.apply(writes)
TacEntityMirror.persist(writes)
}
return null
}
static apply(writes) {
for (const [table, rows] of writes) {
const mirrored = TacEntityMirror.tables.get(table)
if (!mirrored) continue
for (const [id, row] of rows) {
if (row === null) mirrored.delete(id)
else mirrored.set(id, row)
}
}
}
static persist(writes) {
TacEntityMirror.writing = TacEntityMirror.writing
.then(() => TacEntityStorage.commit(writes))
.catch((error) => { console.error(error) })
}
static settled() {
return TacEntityMirror.writing
}
}
class TacStorageHost {
static answer(text) {
try {
return JSON.stringify({ value: TacStorageHost.run(JSON.parse(text)) ?? null })
} catch (error) {
return JSON.stringify({ error: error instanceof TacStorageError ? error.message : 'Tachyon storage failed.' })
}
}
static run(request) {
if (request.op === 'cache.get') return TacCacheStorage.read(request.cache, request.key)
if (request.op === 'cache.set') return TacCacheStorage.write(request.cache, request.key, request.value, request)
if (request.op === 'cache.delete') return TacCacheStorage.remove(request.cache, request.key)
if (request.op === 'entity') return TacEntityMirror.execute(request.command)
throw new TacStorageError(`Tachyon storage has no request named '${request.op}'.`)
}
}
class TacStorage {
static key = Symbol.for('tachyon.tac.storage')
static preloadTimeoutMs = 2000
static loaded = TacStorage.install()
static install() {
Object.defineProperty(globalThis, TacStorage.key, {
configurable: true,
value: Object.freeze({
cache: (name) => TacCacheStorage.adapter(name),
entities: () => TacEntityStorage.adapter,
declare: (table, id, fields) => TacEntityStorage.declare(table, id, fields),
settled: () => Promise.all([TacCacheStorage.settled(), TacEntityMirror.settled()]),
host: (text) => TacStorageHost.answer(text),
mirror: () => TacEntityMirror.preload(),
failure: TacStorageError,
}),
})
return TacCacheStorage.preload()
}
static ready() {
return Promise.race([TacStorage.loaded, new Promise((resolve) => setTimeout(resolve, TacStorage.preloadTimeoutMs))])
}
}
class TacPreferenceBounds {
static mostKeyBytes = 128
static mostValueBytes = 4096
static key(value) {
TacPreferenceBounds.#string(value)
TacPreferenceBounds.#nonempty(value)
TacPreferenceBounds.#length(value, TacPreferenceBounds.mostKeyBytes)
TacPreferenceBounds.#bytes(value, TacPreferenceBounds.mostKeyBytes)
}
static value(value) {
TacPreferenceBounds.#string(value)
TacPreferenceBounds.#length(value, TacPreferenceBounds.mostValueBytes)
TacPreferenceBounds.#bytes(value, TacPreferenceBounds.mostValueBytes)
}
static #string(value) {
if (typeof value !== 'string') throw new TypeError('PreferenceStrings needs raw strings.')
}
static #nonempty(value) {
if (value.length === 0) throw new TypeError('PreferenceStrings needs a nonempty key.')
}
static #length(value, most) {
if (value.length > most) throw new RangeError('PreferenceStrings exceeds its byte limit.')
}
static #bytes(value, most) {
if (new TextEncoder().encode(value).byteLength > most) throw new RangeError('PreferenceStrings exceeds its byte limit.')
}
}
class TacPreferenceStrings {
static read(key) {
return TacPreferenceStrings.#stored(key)
}
static write(key, value) {
return TacPreferenceStrings.#saved(key, value)
}
static #stored(key) {
try {
TacWebApis.allowed('preferenceStrings', false)
TacPreferenceBounds.key(key)
const value = localStorage.getItem(key)
TacPreferenceBounds.value(value)
return value
} catch {
return null
}
}
static #saved(key, value) {
try {
TacWebApis.allowed('preferenceStrings', false)
TacPreferenceBounds.key(key)
TacPreferenceBounds.value(value)
localStorage.setItem(key, value)
return true
} catch {
return false
}
}
}
class TacDocumentTitle {
static mostBytes = 1024
static owned(signal) {
const page = document
return Object.freeze({ set: (...arguments_) => TacDocumentTitle.#write(page, signal, arguments_) })
}
static host(...arguments_) {
const title = TacDocumentTitle.#argument(arguments_)
TacDocumentTitle.#validate(title)
document.title = title
return null
}
static #write(page, signal, arguments_) {
const title = TacDocumentTitle.#argument(arguments_)
TacDocumentTitle.#owner(page, signal)
TacDocumentTitle.#validate(title)
page.title = title
}
static #owner(page, signal) {
if (!signal || signal.aborted || page !== document) throw new Error('DocumentTitle owner is disposed or unavailable.')
}
static #validate(title) {
TacDocumentTitle.#type(title)
TacDocumentTitle.#unicode(title)
TacDocumentTitle.#size(title)
}
static #type(title) {
TacDocumentTitle.#string(title)
TacDocumentTitle.#nul(title)
}
static #argument(arguments_) {
if (arguments_.length !== 1) throw new TypeError('DocumentTitle.set needs exactly one title.')
return arguments_[0]
}
static #string(title) {
if (typeof title !== 'string') throw new TypeError('DocumentTitle needs a string.')
}
static #nul(title) {
if (title.includes('\u0000')) throw new TypeError('DocumentTitle rejects NUL.')
}
static #unicode(title) {
for (const character of title) TacDocumentTitle.#scalar(character.codePointAt(0))
}
static #scalar(point) {
if (point >= 0xd800 && point <= 0xdfff) throw new TypeError('DocumentTitle needs valid Unicode.')
}
static #size(title) {
if (new TextEncoder().encode(title).byteLength > TacDocumentTitle.mostBytes) throw new RangeError('DocumentTitle exceeds 1024 UTF-8 bytes.')
}
}
class TacWebApis {
static key = Symbol.for('tachyon.tac.web')
static mostRandomBytes = 65536
static darkScheme = '(prefers-color-scheme: dark)'
static reducedMotion = '(prefers-reduced-motion: reduce)'
static grants = 'meta[name="tachyon-browser-apis"]'
static positionTimeout = 10000
static smallestWindow = 64
static largestWindow = 16384
static caller = null
static events = null
static clock = Object.freeze({
now: () => Date.now(),
monotonic: () => performance.now(),
timeZone: () => Intl.DateTimeFormat().resolvedOptions().timeZone,
})
static secureRandom = Object.freeze({
bytes: (count) => TacWebApis.randomBytes(count),
uuid: () => TacWebApis.randomUuid(),
})
static connectivity = Object.freeze({ online: () => navigator.onLine })
static pageVisibility = Object.freeze({ visible: () => document.visibilityState === 'visible' })
static preferences = Object.freeze({
scheme: () => TacWebApis.matches(TacWebApis.darkScheme) ? 'dark' : 'light',
motion: () => TacWebApis.matches(TacWebApis.reducedMotion) ? 'reduce' : 'no-preference',
language: () => navigator.language,
})
static preferenceStrings = Object.freeze({
read: (key) => TacPreferenceStrings.read(key),
write: (key, value) => TacPreferenceStrings.write(key, value),
})
static documentTitle = Object.freeze({ set: (...arguments_) => TacDocumentTitle.host(...arguments_) })
static clipboard = Object.freeze({
writeText: (text) => TacWebApis.later({ api: 'clipboard', call: 'writeText', gesture: true }, () => navigator.clipboard.writeText(String(text)), () => null),
readText: () => TacWebApis.later({ api: 'clipboard', call: 'readText', gesture: true }, () => navigator.clipboard.readText(), (text) => ['ClipboardTextRead', { text }]),
})
static share = Object.freeze({
open: (title, text, url) => TacWebApis.later({ api: 'share', call: 'open', gesture: true }, () => navigator.share({ title, text, url }), () => ['ShareFinished', {}]),
})
static notifications = Object.freeze({
permission: () => TacWebApis.permission(),
requestPermission: () => TacWebApis.later({ api: 'notifications', call: 'requestPermission', gesture: true }, () => Notification.requestPermission(), (permission) => ['NotificationPermissionChanged', { permission }]),
show: (title, body) => TacWebApis.notify(title, body),
})
static geolocation = Object.freeze({
currentPosition: () => TacWebApis.later({ api: 'geolocation', call: 'currentPosition', gesture: false }, () => TacWebApis.position(), (position) => TacWebApis.found(position)),
})
static windowControls = Object.freeze({
granted: (control) => TacWebApis.windowGrants().includes(control),
...Object.fromEntries(['minimize', 'maximize', 'fullscreen', 'close'].map((control) => [control, () => TacWebApis.window(control, {})])),
resize: (width, height) => TacWebApis.window('resize', TacWebApis.windowSize(width, height)),
setTitle: (text) => TacWebApis.window('title', { title: String(text) }),
})
static apis = Object.freeze({
clock: TacWebApis.clock,
secureRandom: TacWebApis.secureRandom,
connectivity: TacWebApis.connectivity,
pageVisibility: TacWebApis.pageVisibility,
preferences: TacWebApis.preferences,
preferenceStrings: TacWebApis.preferenceStrings,
documentTitle: TacWebApis.documentTitle,
clipboard: TacWebApis.clipboard,
share: TacWebApis.share,
notifications: TacWebApis.notifications,
geolocation: TacWebApis.geolocation,
windowControls: TacWebApis.windowControls,
})
static installed = TacWebApis.install()
static install() {
Object.defineProperty(globalThis, TacWebApis.key, {
configurable: true,
value: Object.freeze({
...TacWebApis.apis,
host: (text) => TacWebApis.answer(text),
calling: (owner, work) => TacWebApis.calling(owner, work),
}),
})
return true
}
static watch(events) {
TacWebApis.events = events
const connectivity = () => events.publish('ConnectivityChanged', { online: TacWebApis.connectivity.online() })
const visibility = () => events.publish('VisibilityChanged', { visible: TacWebApis.pageVisibility.visible() })
const preferences = () => events.publish('PreferencesChanged', TacWebApis.currentPreferences())
addEventListener('online', connectivity)
addEventListener('offline', connectivity)
addEventListener('languagechange', preferences)
document.addEventListener('visibilitychange', visibility)
TacWebApis.media(TacWebApis.darkScheme)?.addEventListener('change', preferences)
TacWebApis.media(TacWebApis.reducedMotion)?.addEventListener('change', preferences)
connectivity()
visibility()
preferences()
return true
}
static calling(owner, work) {
const previous = TacWebApis.caller
TacWebApis.caller = owner
try {
return work()
} finally {
TacWebApis.caller = previous
}
}
static later({ api, call, gesture }, work, answer) {
TacWebApis.allowed(api, gesture)
const owner = TacWebApis.caller
const failed = (error) => ['BrowserCallFailed', { api, call, message: TacWebApis.message(error) }]
new Promise((resolve) => resolve(work())).then(answer, failed).then((event) => TacWebApis.reply(owner, event))
return null
}
static reply(owner, event) {
if (event) TacWebApis.events?.send(owner, ...event)
}
static allowed(api, gesture) {
const granted = document.querySelector(TacWebApis.grants)?.content.split(' ') ?? []
if (!granted.includes(api)) throw new Error(`The ${api} browser API is not granted. Add "${api}" to tac.browserApis in client/manifest.json.`)
if (gesture && navigator.userActivation?.isActive === false) throw new Error(`The ${api} browser API needs a user action. Call it from a click or key handler.`)
}
static permission() {
TacWebApis.allowed('notifications', false)
return Notification.permission
}
static notify(title, body) {
TacWebApis.allowed('notifications', false)
if (Notification.permission !== 'granted') throw new Error('Notifications are not allowed. Call requestPermission first.')
new Notification(String(title), { body: String(body ?? '') })
return null
}
static windowGrants() {
return Array.isArray(globalThis.__tachyonWindowControls) ? globalThis.__tachyonWindowControls : []
}
static window(control, payload) {
if (typeof globalThis.__tachyonNativeHostCall !== 'function') throw new Error(`The ${control} window control needs the native app window.`)
if (!TacWebApis.windowGrants().includes(control)) throw new Error(`The ${control} window control is not granted. Add "${control}" to tac.application.window.controls in client/manifest.json.`)
void Promise.resolve(globalThis.__tachyonNativeHostCall(`window.${control}`, JSON.stringify(payload))).catch(() => {})
return null
}
static windowSize(width, height) {
const { smallestWindow: least, largestWindow: most } = TacWebApis
if (![width, height].every((size) => Number.isInteger(size) && size >= least && size <= most)) throw new RangeError(`A window size is a whole number from ${least} to ${most}.`)
return { width, height }
}
static position() {
return new Promise((resolve, reject) => {
navigator.geolocation.getCurrentPosition(resolve, reject, { timeout: TacWebApis.positionTimeout })
})
}
static found({ coords, timestamp }) {
return ['PositionFound', { latitude: coords.latitude, longitude: coords.longitude, accuracy: coords.accuracy, timestamp }]
}
static currentPreferences() {
const { scheme, motion, language } = TacWebApis.preferences
return { scheme: scheme(), motion: motion(), language: language() }
}
static media(query) {
return typeof matchMedia === 'function' ? matchMedia(query) : undefined
}
static matches(query) {
return TacWebApis.media(query)?.matches === true
}
static message(error) {
return typeof error?.message === 'string' && error.message ? error.message : 'Tachyon browser call failed.'
}
static answer(text) {
try {
return JSON.stringify({ value: TacWebApis.run(JSON.parse(text)) })
} catch (error) {
return JSON.stringify({ error: TacWebApis.message(error) })
}
}
static run(request) {
const api = Object.hasOwn(TacWebApis.apis, request?.api) ? TacWebApis.apis[request.api] : undefined
const call = api && Object.hasOwn(api, request.call) ? api[request.call] : undefined
if (typeof call !== 'function') throw new Error(`Tachyon has no browser call named '${request?.api}.${request?.call}'.`)
return call(...(Array.isArray(request.args) ? request.args : []))
}
static randomBytes(count) {
if (!Number.isInteger(count) || count < 0 || count > TacWebApis.mostRandomBytes) {
throw new RangeError(`SecureRandom gives 0 to ${TacWebApis.mostRandomBytes} bytes.`)
}
return [...crypto.getRandomValues(new Uint8Array(count))]
}
static randomUuid() {
const bytes = crypto.getRandomValues(new Uint8Array(16))
bytes[6] = (bytes[6] & 0x0f) | 0x40
bytes[8] = (bytes[8] & 0x3f) | 0x80
const hex = [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('')
return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}
}
class TacBrowserHttp {
static key = Symbol.for('tachyon.tac.http')
static mostRequests = 64
static mostBytes = 1048576
static requestBytes = 65536
static pending = 0
static installed = TacBrowserHttp.install()
static install() {
Object.defineProperty(globalThis, TacBrowserHttp.key, {
configurable: true,
value: Object.freeze({ start: (text) => TacBrowserHttp.start(text) }),
})
}
static start(text) {
const controller = new AbortController()
return Object.freeze({ answer: TacBrowserHttp.#answer(text, controller), cancel: () => controller.abort('CANCELLED') })
}
static async #answer(text, controller) {
try {
const request = TacBrowserHttp.#request(text)
const value = await TacBrowserHttp.#exchange(request, controller)
const answer = JSON.stringify({ value })
if (new TextEncoder().encode(answer).length > TacBrowserHttp.mostBytes) TacBrowserHttp.#fail('HTTP_LIMIT')
return answer
} catch (error) {
return JSON.stringify({ error: { code: TacBrowserHttp.#code(error, controller) } })
}
}
static #code(error, controller) {
if (controller.signal.aborted) return controller.signal.reason === 'DEADLINE' ? 'DEADLINE' : 'CANCELLED'
const allowed = ['HTTP_REQUEST', 'HTTP_ORIGIN', 'HTTP_HEADER', 'HTTP_QUERY', 'HTTP_BODY', 'HTTP_LIMIT', 'HTTP_RESPONSE', 'HTTP_STATUS', 'HTTP_CAPACITY']
return allowed.includes(error?.code) ? error.code : 'HTTP_NETWORK'
}
static #fail(code) { throw Object.assign(new Error('Browser HTTP failed.'), { code }) }
static #request(text) {
if (typeof text !== 'string' || new TextEncoder().encode(text).length > TacBrowserHttp.requestBytes) TacBrowserHttp.#fail('HTTP_REQUEST')
let request
try { request = JSON.parse(text) } catch { TacBrowserHttp.#fail('HTTP_REQUEST') }
if (!request || Array.isArray(request) || typeof request !== 'object') TacBrowserHttp.#fail('HTTP_REQUEST')
const allowed = ['method', 'path', 'query', 'headers', 'body', 'redirect', 'credentials', 'deadline', 'maxResponseBytes', 'publicRead', 'revalidate']
if (Object.keys(request).some((key) => !allowed.includes(key))) TacBrowserHttp.#fail('HTTP_REQUEST')
if (!['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method)) TacBrowserHttp.#fail('HTTP_REQUEST')
if (request.redirect !== 'error' || request.credentials !== 'omit') TacBrowserHttp.#fail('HTTP_REQUEST')
if (!Number.isSafeInteger(request.deadline) || request.deadline > Date.now() + 60000) TacBrowserHttp.#fail('HTTP_REQUEST')
if (!Number.isInteger(request.maxResponseBytes) || request.maxResponseBytes < 1 || request.maxResponseBytes > TacBrowserHttp.mostBytes) TacBrowserHttp.#fail('HTTP_LIMIT')
if (request.publicRead !== undefined && typeof request.publicRead !== 'boolean') TacBrowserHttp.#fail('HTTP_REQUEST')
if (request.revalidate !== undefined && typeof request.revalidate !== 'boolean') TacBrowserHttp.#fail('HTTP_REQUEST')
const url = TacBrowserHttp.#url(request)
const headers = TacBrowserHttp.#headers(request.headers)
const body = TacBrowserHttp.#body(request, headers)
const cached = request.method === 'GET' && request.publicRead === true && !headers.has('authorization')
return { url, headers, body, method: request.method, deadline: request.deadline, limit: request.maxResponseBytes,
cache: cached ? request.revalidate === true ? 'no-cache' : 'default' : 'no-store' }
}
static #url(request) {
const path = request.path
if (typeof path !== 'string' || !path.startsWith('/') || path.startsWith('//') || /[\s\\?#%]/.test(path) || path.split('/').some((part) => part === '.' || part === '..') || new TextEncoder().encode(path).length > 2048) TacBrowserHttp.#fail('HTTP_ORIGIN')
const url = new URL(path, location.href)
if (url.origin !== location.origin) TacBrowserHttp.#fail('HTTP_ORIGIN')
const query = request.query ?? {}
if (!query || typeof query !== 'object' || Array.isArray(query)) TacBrowserHttp.#fail('HTTP_QUERY')
for (const [key, value] of Object.entries(query)) {
if (key.length > 256 || !['string', 'number', 'boolean'].includes(typeof value) || new TextEncoder().encode(JSON.stringify(value)).length > 8192) TacBrowserHttp.#fail('HTTP_QUERY')
TacBrowserHttp.#json(value)
url.searchParams.set(key, String(value))
}
if (new TextEncoder().encode(url.href).length > 8192) TacBrowserHttp.#fail('HTTP_QUERY')
return url.href
}
static #headers(values = {}) {
if (!values || typeof values !== 'object' || Array.isArray(values) || Object.keys(values).length > 128) TacBrowserHttp.#fail('HTTP_HEADER')
const headers = new Headers()
const refused = ['host', 'connection', 'content-length', 'transfer-encoding', 'cookie', 'origin', 'referer']
for (const [key, value] of Object.entries(values)) {
if (!/^[!#$%&'*+.^_`|~0-9A-Za-z-]{1,128}$/.test(key) || refused.includes(key.toLowerCase()) || key.toLowerCase().startsWith('sec-') || key.toLowerCase().startsWith('proxy-') || typeof value !== 'string' || /[\r\n\0]/.test(value) || new TextEncoder().encode(value).length > 8192) TacBrowserHttp.#fail('HTTP_HEADER')
headers.set(key, value)
}
return headers
}
static #body(request, headers) {
if (!Object.hasOwn(request, 'body')) return undefined
if (request.method === 'GET' || request.method === 'HEAD') TacBrowserHttp.#fail('HTTP_BODY')
TacBrowserHttp.#json(request.body)
headers.set('content-type', 'application/json')
return JSON.stringify(request.body)
}
static #json(value, depth = 0) {
if (depth > 32) TacBrowserHttp.#fail('HTTP_RESPONSE')
if (typeof value === 'number' && (!Number.isFinite(value) || Math.abs(value) > Number.MAX_SAFE_INTEGER)) TacBrowserHttp.#fail('HTTP_RESPONSE')
if (value && typeof value === 'object') {
for (const item of Object.values(value)) TacBrowserHttp.#json(item, depth + 1)
}
}
static async #exchange(request, controller) {
if (TacBrowserHttp.pending >= TacBrowserHttp.mostRequests) TacBrowserHttp.#fail('HTTP_CAPACITY')
TacBrowserHttp.pending += 1
const remaining = request.deadline - Date.now()
const timer = setTimeout(() => controller.abort('DEADLINE'), Math.max(0, remaining))
try {
if (remaining <= 0) { controller.abort('DEADLINE'); TacBrowserHttp.#fail('HTTP_REQUEST') }
const response = await fetch(request.url, { method: request.method, headers: request.headers, body: request.body,
credentials: 'omit', redirect: 'error', mode: 'same-origin', cache: request.cache, signal: controller.signal })
if (!response.ok) TacBrowserHttp.#fail('HTTP_STATUS')
const text = await TacBrowserHttp.#read(response, request.limit)
if (controller.signal.aborted) TacBrowserHttp.#fail('HTTP_REQUEST')
if (!text.trim()) return null
let value
try { value = JSON.parse(text) } catch { TacBrowserHttp.#fail('HTTP_RESPONSE') }
TacBrowserHttp.#json(value)
return value
} finally { clearTimeout(timer); TacBrowserHttp.pending -= 1 }
}
static async #read(response, limit) {
if (!response.body) return ''
const reader = response.body.getReader()
const decoder = new TextDecoder('utf-8', { fatal: true })
let length = 0
let text = ''
try {
while (true) {
const { done, value } = await reader.read()
if (done) break
length += value.byteLength
if (length > limit) TacBrowserHttp.#fail('HTTP_LIMIT')
try { text += decoder.decode(value, { stream: true }) } catch { TacBrowserHttp.#fail('HTTP_RESPONSE') }
}
} catch (error) {
await reader.cancel().catch(() => {})
throw error
} finally { reader.releaseLock() }
try { return text + decoder.decode() } catch { TacBrowserHttp.#fail('HTTP_RESPONSE') }
}
}
class TacNativeCapabilities {
static statuses = new Set(['available', 'selected', 'cancelled', 'read', 'released', 'denied', 'unsupported', 'unavailable', 'invalid', 'disposed', 'tooLarge'])
static filePicker(signal) {
const owner = TacWebApis.randomUuid()
let disposed = signal?.aborted ?? false
const call = async (operation, values = {}) => {
if (disposed) return { status: 'disposed' }
if (globalThis.__tachyonNativeFilePicker !== true) return { status: 'unsupported' }
if (!globalThis.__tachyonNativeGrants?.includes('filePicker')) return { status: 'denied' }
try {
const answer = await globalThis.__tachyonNativeHostCall(`filePicker.${operation}`, JSON.stringify({ owner, ...values }))
if (disposed) return { status: 'disposed' }
const result = JSON.parse(answer)
return result && TacNativeCapabilities.statuses.has(result.status) ? result : { status: 'unavailable' }
} catch {
return { status: 'unavailable' }
}
}
signal?.addEventListener('abort', () => {
disposed = true
Promise.resolve().then(() => globalThis.__tachyonNativeHostCall?.('filePicker.dispose', JSON.stringify({ owner }))).catch(() => {})
}, { once: true })
return Object.freeze({
available: () => !disposed && globalThis.__tachyonNativeFilePicker === true && globalThis.__tachyonNativeGrants?.includes('filePicker') === true,
open: (options = {}) => !options || typeof options !== 'object' || (options.multiple !== undefined && options.multiple !== false) || (options.types && (!Array.isArray(options.types) || options.types.some((type) => !['text/plain', 'application/json'].includes(type))))
? Promise.resolve({ status: 'invalid' }) : call('open', { types: options.types ?? [] }),
readText: (handle) => call('readText', { handle }),
release: (handle) => call('release', { handle }),
})
}
}
class AuthoredRootUpdate {
static _htmlNamespace = 'http://www.w3.org/1999/xhtml'
static _svgNamespace = 'http://www.w3.org/2000/svg'
static _componentTag = 'tachyon-component'
static _componentKind = 'component'
static _editingControls = 'textarea, [contenteditable]'
static _defaultSlot = ''
_container
_source
_active
_editingAtStart
_retained = new Map()
_refused = new Set()
_pairs = []
_children = new Map()
_texts = []
constructor(container, source, active, editingAtStart) {
this._container = container
this._source = source
this._active = active
this._editingAtStart = editingAtStart
}
prepare() {
const previous = AuthoredRootUpdate._recordedChildren(this._container)
const desired = AuthoredRootUpdate._recordedChildren(this._source)
this._planChildren({ parent: this._container, previous, desired })
return this
}
retains(element) {
for (const old of this._retained.values()) {
if (old === element) return true
}
return false
}
refuses(chain) {
return chain.some((pair) => this._refused.has(pair.old) && AuthoredRootUpdate._isCustom(pair.old))
}
commit() {
for (const pair of this._pairs) this._syncElement(pair)
for (const text of this._texts) this._syncText(text)
for (const [parent, desired] of this._children) this._commitGroup(parent, desired)
this._recordElements()
}
static _recordedChildren(parent) {
const record = Elements.records.get(parent)
if (record) return record.children
return [...parent.childNodes]
}
static _isCustom(element) {
return element.localName.includes('-')
}
static _slotName(node) {
const record = Elements.records.get(node)
return record?.attributes.get('slot') ?? AuthoredRootUpdate._defaultSlot
}
static _sameChildren(current, previous) {
if (current.length !== previous.length) return false
return previous.every((child, index) => child === current[index])
}
static _byPath(children) {
const index = new Map()
for (const child of children) {
const record = Elements.records.get(child)
if (record) index.set(record.path, child)
}
return index
}
_planChildren(group) {
this._children.set(group.parent, group.desired)
const previousByPath = AuthoredRootUpdate._byPath(group.previous)
for (let index = 0; index < group.desired.length; index += 1) {
const fresh = group.desired[index]
if (fresh.nodeType === Node.TEXT_NODE) {
this._planText(group.parent, group.previous[index], fresh)
continue
}
const record = Elements.records.get(fresh)
if (!record) continue
const old = previousByPath.get(record.path)
if (!old) continue
const pair = retainedPair(old)
if (pair) this._planPair(group.parent, fresh, pair)
}
}
_planText(parent, old, fresh) {
if (old?.nodeType !== Node.TEXT_NODE) return
if (old.parentNode !== parent) return
this._retained.set(fresh, old)
this._texts.push({ old, fresh })
}
_planPair(parent, fresh, pair) {
const layout = this._samePlace(parent, fresh, pair) ? this._layout(pair) : null
if (!layout) {
this._refused.add(pair.old)
return
}
this._retained.set(pair.fresh, pair.old)
this._pairs.push(pair)
for (const group of layout) this._planChildren(group)
}
_samePlace(parent, fresh, pair) {
if (pair.fresh !== fresh) return false
return pair.old.parentNode === parent
}
_layout(pair) {
if (changesObservedAttribute(pair)) return null
if (!this._retainable(pair)) return null
const slots = this._lightSlots(pair)
if (slots.length) return this._slotLayout(pair, slots)
return this._directLayout(pair)
}
_retainable(pair) {
const element = pair.old
if (element.namespaceURI === AuthoredRootUpdate._svgNamespace) return true
if (element.namespaceURI !== AuthoredRootUpdate._htmlNamespace) return false
if (element.localName === AuthoredRootUpdate._componentTag) {
return pair.previous.node.k === AuthoredRootUpdate._componentKind
}
return !element.matches(AuthoredRootUpdate._editingControls)
}
_lightSlots(pair) {
if (pair.previous.node.k === AuthoredRootUpdate._componentKind) return []
if (!AuthoredRootUpdate._isCustom(pair.old)) return []
return ownedSlots(pair.old)
}
_directLayout(pair) {
const current = [...pair.old.childNodes]
if (!AuthoredRootUpdate._sameChildren(current, pair.previous.children)) return null
const desired = Elements.records.get(pair.fresh).children
return [{ parent: pair.old, previous: pair.previous.children, desired }]
}
_slotLayout(pair, slots) {
const groups = new Map()
for (const slot of slots) {
const name = slot.getAttribute('name') ?? AuthoredRootUpdate._defaultSlot
groups.set(name, { parent: slot, previous: [], desired: [] })
}
if (groups.size !== slots.length) return null
const desired = Elements.records.get(pair.fresh).children
if (!this._assign(pair.previous.children, groups, 'previous')) return null
if (!this._assign(desired, groups, 'desired')) return null
if (!this._slotsUnchanged(groups)) return null
if (!this._sameAssignments(pair.previous.children, desired)) return null
return [...groups.values()]
}
_assign(children, groups, side) {
for (const child of children) {
const group = groups.get(AuthoredRootUpdate._slotName(child))
if (!group) return false
group[side].push(child)
}
return true
}
_slotsUnchanged(groups) {
for (const group of groups.values()) {
const current = [...group.parent.childNodes]
if (!AuthoredRootUpdate._sameChildren(current, group.previous)) return false
if (!group.previous.every((child) => this._keepsSlotAttribute(child))) return false
}
return true
}
_keepsSlotAttribute(child) {
const current = child.getAttribute?.('slot') ?? AuthoredRootUpdate._defaultSlot
return AuthoredRootUpdate._slotName(child) === current
}
_sameAssignments(previous, desired) {
const desiredByPath = AuthoredRootUpdate._byPath(desired)
for (const child of previous) {
const path = Elements.records.get(child)?.path
const fresh = desiredByPath.get(path)
if (fresh && AuthoredRootUpdate._slotName(child) !== AuthoredRootUpdate._slotName(fresh)) return false
}
return true
}
_syncElement(pair) {
const next = Elements.records.get(pair.fresh)
syncAuthoredAttributes(pair.old, pair.previous, next)
syncControlValue(pair.old, pair.fresh, next, this._active, this._editingAtStart)
syncControlFlags(pair.old, pair.fresh, pair.previous, next)
transferEventListeners(pair.old, pair.fresh)
if (pair.fresh.tachyonComponent) pair.old.tachyonComponent = pair.fresh.tachyonComponent
}
_syncText(text) {
if (text.old.data !== text.fresh.data) text.old.data = text.fresh.data
}
_commitGroup(parent, desired) {
const children = desired.map((child) => this._resolve(child))
const wanted = new Set(children)
for (const child of [...parent.childNodes]) {
if (!wanted.has(child)) child.remove()
}
let cursor = parent.firstChild
for (const child of children) {
if (child !== cursor) parent.insertBefore(child, cursor)
cursor = child.nextSibling
}
}
_resolve(child) {
return this._retained.get(child) ?? child
}
_recordElements() {
for (const pair of this._pairs) {
const next = Elements.records.get(pair.fresh)
Elements.records.set(pair.old, { ...next, children: next.children.map((child) => this._resolve(child)) })
}
for (const [path, fresh] of Rendering.elements) {
const element = this._resolve(fresh)
Rendering.elements.set(path, element)
const record = Elements.records.get(element)
record.children = record.children.map((child) => this._resolve(child))
}
Rendering.mounts = Rendering.mounts.map(([host, ...rest]) => [this._resolve(host), ...rest])
}
}
const readViewPlan = () => {
const planElement = document.getElementById('tachyon-view')
return planElement ? JSON.parse(planElement.textContent || '{}') : null
}
class ClientView {
static page = readViewPlan()
static plan = ClientView.page ?? { nodes: [], services: {}, state: {} }
}
class Components {
static instances = new Map()
static records = new Map()
static mountSchedules = new Map()
static activations = new WeakMap()
static activating = new Set()
}
const cancelMount = (key) => Components.mountSchedules.get(key)?.cancel()
class Page {
static owner = Object.assign(Object.create(null), ClientView.plan.state)
}
class Rendering {
static current = null
static renderAgain = false
static nodeCount = 0
static iterationCount = 0
static instanceKeys = new Set()
static elements = new Map()
static mounts = []
}
class Elements {
static records = new WeakMap()
static listeners = new WeakMap()
static #scopeProperty = '--_tac-owner'
static #htmlNamespace = 'http://www.w3.org/1999/xhtml'
static #svgNamespace = 'http://www.w3.org/2000/svg'
static #svgTag = 'svg'
static #foreignObjectTag = 'foreignObject'
static #xlinkNamespace = 'http://www.w3.org/1999/xlink'
static #xmlNamespace = 'http://www.w3.org/XML/1998/namespace'
static #xmlnsNamespace = 'http://www.w3.org/2000/xmlns/'
static rootContext = { slot: null, namespace: Elements.#htmlNamespace }
static #htmlIntegrationPoints = [Elements.#foreignObjectTag, 'desc', 'title']
static #svgTags = new Map([
'altGlyph', 'altGlyphDef', 'altGlyphItem', 'animateColor', 'animateMotion',
'animateTransform', 'clipPath', 'feBlend', 'feColorMatrix', 'feComponentTransfer',
'feComposite', 'feConvolveMatrix', 'feDiffuseLighting', 'feDisplacementMap',
'feDistantLight', 'feDropShadow', 'feFlood', 'feFuncA', 'feFuncB', 'feFuncG',
'feFuncR', 'feGaussianBlur', 'feImage', 'feMerge', 'feMergeNode', 'feMorphology',
'feOffset', 'fePointLight', 'feSpecularLighting', 'feSpotLight', 'feTile',
'feTurbulence', Elements.#foreignObjectTag, 'glyphRef', 'linearGradient', 'radialGradient',
'textPath',
].map(name => /** @type {[string, string]} */ ([name.toLowerCase(), name])))
static #svgAttributes = new Map([
'attributeName', 'attributeType', 'baseFrequency', 'baseProfile', 'calcMode',
'clipPathUnits', 'diffuseConstant', 'edgeMode', 'filterUnits', 'glyphRef',
'gradientTransform', 'gradientUnits', 'kernelMatrix', 'kernelUnitLength',
'keyPoints', 'keySplines', 'keyTimes', 'lengthAdjust', 'limitingConeAngle',
'markerHeight', 'markerUnits', 'markerWidth', 'maskContentUnits', 'maskUnits',
'numOctaves', 'pathLength', 'patternContentUnits', 'patternTransform',
'patternUnits', 'pointsAtX', 'pointsAtY', 'pointsAtZ', 'preserveAlpha',
'preserveAspectRatio', 'primitiveUnits', 'refX', 'refY', 'repeatCount',
'repeatDur', 'requiredExtensions', 'requiredFeatures', 'specularConstant',
'specularExponent', 'spreadMethod', 'startOffset', 'stdDeviation', 'stitchTiles',
'surfaceScale', 'systemLanguage', 'tableValues', 'targetX', 'targetY',
'textLength', 'viewBox', 'viewTarget', 'xChannelSelector', 'yChannelSelector',
'zoomAndPan',
].map(name => /** @type {[string, string]} */ ([`${Elements.#svgNamespace}:${name.toLowerCase()}`, name])))
static #foreignAttributes = new Map([
['xlink:actuate', Elements.#xlinkNamespace],
['xlink:arcrole', Elements.#xlinkNamespace],
['xlink:href', Elements.#xlinkNamespace],
['xlink:role', Elements.#xlinkNamespace],
['xlink:show', Elements.#xlinkNamespace],
['xlink:title', Elements.#xlinkNamespace],
['xlink:type', Elements.#xlinkNamespace],
['xml:lang', Elements.#xmlNamespace],
['xml:space', Elements.#xmlNamespace],
['xmlns', Elements.#xmlnsNamespace],
['xmlns:xlink', Elements.#xmlnsNamespace],
].map(([name, namespace]) => /** @type {[string, string]} */ ([`${Elements.#svgNamespace}:${name}`, namespace])))
static create(tag, namespace) {
const svg = Elements.#usesSvg(tag, namespace)
const adjusted = Elements.#adjustedTag(tag)
return Elements.#createElement(tag, adjusted, svg)
}
static childContext(element, context) {
const namespace = Elements.#childNamespace(element)
return { slot: context.slot, namespace }
}
static setAttribute(element, name, value) {
const namespace = Elements.#attributeNamespace(element, name)
const adjusted = Elements.#adjustedAttribute(element, name)
Elements.#writeAttribute(element, namespace, adjusted, value)
}
static removeAttribute(element, name) {
const namespace = Elements.#attributeNamespace(element, name)
const adjusted = Elements.#adjustedAttribute(element, name)
const local = name.slice(name.indexOf(':') + 1)
Elements.#deleteAttribute(element, namespace, adjusted, local)
}
static #usesSvg(tag, namespace) {
return namespace === Elements.#svgNamespace || tag === Elements.#svgTag
}
static #adjustedTag(tag) {
return Elements.#svgTags.get(tag) ?? tag
}
static #createElement(tag, adjusted, svg) {
if (svg) return document.createElementNS(Elements.#svgNamespace, adjusted)
return document.createElement(tag)
}
static #childNamespace(element) {
if (element.namespaceURI === Elements.#svgNamespace && !Elements.#htmlIntegrationPoints.includes(element.localName)) return Elements.#svgNamespace
return Elements.#htmlNamespace
}
static #attributeNamespace(element, name) {
return Elements.#foreignAttributes.get(`${element.namespaceURI}:${name}`) ?? null
}
static #adjustedAttribute(element, name) {
return Elements.#svgAttributes.get(`${element.namespaceURI}:${name}`) ?? name
}
static #writeAttribute(element, namespace, name, value) {
if (namespace) {
element.setAttributeNS(namespace, name, value)
return
}
element.setAttribute(name, value)
}
static #deleteAttribute(element, namespace, name, local) {
if (namespace) {
element.removeAttributeNS(namespace, local)
return
}
element.removeAttribute(name)
}
static markScope(host, name) {
host.setAttribute('data-tac-scope', name)
host.style.setProperty(Elements.#scopeProperty, name)
}
}
class NativeState {
static writes = Promise.resolve()
static writeCount = 0
static methods = new WeakSet()
static owners = new Map()
static startUpRequestLimit = 1048576
static memberRequestLimit = 65536
static tooLarge(request, service, payload, limit) {
const subject = request.op === 'init' ? 'start-up request' : `request to ${request.name ?? request.op}`
const size = new TextEncoder().encode(payload).byteLength
return `Native service ${service}: The ${subject} is ${size} bytes. The limit is ${limit} bytes. Pass less data to the service.`
}
}
class SharedData {
static root = Object.freeze(Object.create(null))
static modules = new Map()
}
const readSharedData = (name) => {
if (!/^[A-Za-z_][A-Za-z0-9_]{0,127}$/.test(name)) return undefined
if (!SharedData.modules.has(name)) {
const url = new URL(`../shared/data/${name}.json`, import.meta.url)
SharedData.modules.set(name, import(url.href, { with: { type: 'json' } }).then((module) => module.default))
}
return SharedData.modules.get(name)
}
const markDocument = (name, value) => {
document.documentElement.dataset[name] = value
}
const pageContext = () => Object.freeze({
path: location.pathname.replace(/(.)\/+$/, '$1'),
development: document.body?.dataset.tachyonEnvironment === 'development',
})
const evaluate = async (node, environment) => {
if (!node || typeof node !== 'object') return undefined
switch (node.k) {
case 'await': return await evaluate(node.e, environment)
case 'lit': return node.v
case 'id':
if (node.n in environment.locals) return environment.locals[node.n]
if (node.n === 'data') return SharedData.root
if (node.n === 'page') return pageContext()
return environment.owner?.[node.n]
case 'get': {
const target = await evaluate(node.o, environment)
if (target === SharedData.root) return await readSharedData(node.p)
return target === null || target === undefined ? undefined : target[node.p]
}
case 'idx': {
const target = await evaluate(node.o, environment)
return Array.isArray(target) ? target[node.i] : undefined
}
case 'not': return !await evaluate(node.e, environment)
case 'cmp': {
const left = await evaluate(node.l, environment)
const right = await evaluate(node.r, environment)
if (node.op === 'eq') return left === right
if (node.op === 'ne') return left !== right
if (node.op === 'lt') return left < right
if (node.op === 'le') return left <= right
if (node.op === 'gt') return left > right
return left >= right
}
case 'log': {
const left = await evaluate(node.l, environment)
if (node.op === 'and') return left ? await evaluate(node.r, environment) : left
return left || await evaluate(node.r, environment)
}
case 'num': {
const left = await evaluate(node.l, environment)
const right = await evaluate(node.r, environment)
if (node.op === 'add') return left + right
if (node.op === 'sub') return left - right
if (node.op === 'mul') return left * right
return left / right
}
case 'if':
return await evaluate(node.c, environment)
? await evaluate(node.t, environment)
: await evaluate(node.f, environment)
case 'call': {
const callee = node.c
const owner = callee.k === 'get' || callee.k === 'idx'
? await evaluate(callee.o, environment)
: environment.owner
const target = await evaluate(callee, environment)
if (typeof target !== 'function') return undefined
const arguments_ = await Promise.all((node.a || []).map((value) => evaluate(value, environment)))
return target.apply(owner, arguments_)
}
default: return undefined
}
}
const display = (value) => {
if (value === null || value === undefined || typeof value === 'object') return ''
return String(value)
}
const childEnvironment = (environment, locals) => ({
owner: environment.owner,
locals: Object.assign(Object.create(environment.locals), locals),
})
const memberTarget = (owner, path) => {
const dot = typeof path === 'string' ? path.indexOf('.') : -1
if (dot < 0) return [owner, path]
return [owner?.[path.slice(0, dot)], path.slice(dot + 1)]
}
const ownedEnvironment = (owner) => ({ owner, locals: Object.create(null) })
const resolveEventArgument = async (argument, event, environment) => {
if ('value' in argument) return argument.value
if ('expression' in argument) return evaluate(argument.expression, environment)
if (!('event' in argument) || argument.event === '') return event
let value = event
for (const segment of String(argument.event).split('.')) {
if (value === null || value === undefined) return undefined
value = value[segment]
}
return value
}
const bindEvent = (element, type, binding, environment) => {
const listener = async (event) => {
try {
const arguments_ = await Promise.all(
(binding.arguments || []).map((argument) => resolveEventArgument(argument, event, environment)),
)
if (binding.assign) {
const { target, operator } = binding.assign
const [holder, field] = memberTarget(environment.owner, target)
const current = holder?.[field]
const value = arguments_[0]
holder[field] =
operator === '+=' ? current + value
: operator === '-=' ? current - value
: operator === '*=' ? current * value
: operator === '/=' ? current / value
: value
} else {
const [holder, name] = memberTarget(environment.owner, binding.handler)
const handler = holder?.[name]
if (typeof handler !== 'function') throw new TypeError(`Tac handler '${binding.handler}' is not available.`)
await handler.call(holder, ...(NativeState.methods.has(handler) ? arguments_ : [event, ...arguments_]))
}
await render()
markDocument('tachyonEvents', 'handled')
} catch {
markDocument('tachyonEventError', 'handler_failed')
}
}
element.addEventListener(type, listener)
const listeners = Elements.listeners.get(element) || []
listeners.push({ type, listener })
Elements.listeners.set(element, listeners)
}
const applyAttributes = async (element, attributes, environment) => {
for (const attribute of attributes || []) {
if (attribute.event) {
bindEvent(element, attribute.eventType, attribute.event, environment)
continue
}
const value = 'expression' in attribute
? await evaluate(attribute.expression, environment)
: attribute.value
if (value === false || value === null || value === undefined) {
Elements.removeAttribute(element, attribute.name)
continue
}
const rendered = value === true ? '' : display(value)
Elements.setAttribute(element, attribute.name, rendered)
if (['checked', 'disabled', 'selected'].includes(attribute.name)) element[attribute.name] = Boolean(value)
else if (attribute.name === 'value' && 'value' in element) {
try { element.value = rendered } catch {}
}
}
}
class StaticCache {
static namePrefix = 'tachyon-static-'
}
const openStaticCache = async () => {
try {
if (!globalThis.caches) return null
const name = (await caches.keys()).find((value) => value.startsWith(StaticCache.namePrefix))
return name ? await caches.open(name) : null
} catch { return null }
}
const storableResponse = (response) => response.ok && response.type === 'basic'
&& !/(?:no-store|private)/i.test(response.headers.get('cache-control') || '')
&& !response.headers.get('vary')
const precacheAssets = async (urls) => {
const cache = await openStaticCache()
if (!cache) return
if (!Array.isArray(urls) || urls.length > 512) throw new RangeError('Tac precache exceeds 512 assets.')
const deadline = Date.now() + 30000
let warmedBytes = 0
for (const raw of new Set(urls)) {
if (Date.now() >= deadline || warmedBytes >= 33554432) break
try {
const url = new URL(raw, location.href)
if (url.origin !== location.origin || !['http:', 'https:'].includes(url.protocol)) continue
if (await cache.match(url)) continue
const response = await fetch(url, { credentials: 'omit', signal: AbortSignal.timeout(5000) })
const body = await responseBody(response, 4194304)
if (body === null || !storableResponse(response)) continue
if (warmedBytes + body.byteLength > 33554432) break
warmedBytes += body.byteLength
await cache.put(url, new Response(body, { status: response.status, statusText: response.statusText, headers: response.headers }))
} catch {}
}
}
const clearStaticCache = async () => {
try {
if (!globalThis.caches) return
for (const name of await caches.keys()) {
if (name.startsWith(StaticCache.namePrefix)) await caches.delete(name)
}
} catch {}
}
const planAssets = (nodes, found = []) => {
for (const node of nodes || []) {
if (found.length >= 512) break
if (node.k === 'component') {
for (const service of node.services || []) if (service.module) found.push(service.module)
planAssets(node.template, found)
planAssets(node.slot, found)
for (const nodes of Object.values(node.slots || {})) planAssets(nodes, found)
}
planAssets(node.children, found)
}
return found
}
const responseBody = async (response, limit) => {
if (!response.body) return new ArrayBuffer(0)
const reader = response.body.getReader()
const chunks = []
let length = 0
let timedOut = false
const timer = setTimeout(() => { timedOut = true; void reader.cancel().catch(() => {}) }, 2000)
try {
while (true) {
const { done, value } = await reader.read()
if (done) break
length += value.byteLength
if (length > limit) { void reader.cancel().catch(() => {}); return null }
chunks.push(value)
}
if (timedOut) return null
const body = new Uint8Array(length)
let offset = 0
for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.byteLength }
return body.buffer
} finally { clearTimeout(timer) }
}
const clientRequest = (input, init = {}, options = {}) => {
const request = new Request(typeof input === 'string' ? new URL(input, location.href) : input, init)
return fetch(new Request(request, { cache: options.publicRead !== true ? 'no-store' : options.revalidate === true ? 'no-cache' : 'default' }))
}
const webStorage = (persistent) => {
try { return persistent ? localStorage : sessionStorage } catch { return null }
}
const fitsStorage = (raw) => raw.length <= 65536 && new TextEncoder().encode(raw).byteLength <= 65536
const readStoredField = (store, key, fallback) => {
try {
const raw = store.getItem(key)
if (raw === null || !fitsStorage(raw)) return fallback
return JSON.parse(raw)
} catch { return fallback }
}
const writeStoredField = (store, key, value) => {
try {
if (value === undefined) { store.removeItem(key); return }
const raw = JSON.stringify(value)
if (raw !== undefined && fitsStorage(raw)) store.setItem(key, raw)
} catch {}
}
class Owners {
static bound = new WeakSet()
static managed = new WeakMap()
static modules = new WeakMap()
static stored = new WeakMap()
static adoptLegacyField(store, key, legacy) {
try {
const raw = store.getItem(legacy)
if (raw === null) return
store.removeItem(legacy)
if (store.getItem(key) === null) store.setItem(key, raw)
} catch {}
}
}
class StoredAccessor {
constructor(owner, read, store, key) {
this.owner = owner
this.read = read
this.store = store
this.key = key
this.saved = JSON.stringify(read.call(owner))
}
save() {
try {
const value = this.read.call(this.owner)
const raw = JSON.stringify(value)
if (raw === this.saved) return
this.saved = raw
writeStoredField(this.store, this.key, value)
} catch {}
}
}
const bindPersistentFields = (owner, scope) => {
if (Owners.bound.has(owner)) return owner
Owners.bound.add(owner)
const stored = Owners.stored.get(owner) || {}
const accessors = []
for (const name of Object.keys(owner)) {
if (!Object.hasOwn(stored, name) || !Object.getOwnPropertyDescriptor(owner, name)?.configurable) continue
const local = stored[name] === 'local'
const store = webStorage(local)
if (!store) continue
const key = `tac:${scope}:${name}`
Owners.adoptLegacyField(store, key, `tac:${scope}:${local ? '$$' : '$'}${name}`)
const { get, set } = Object.getOwnPropertyDescriptor(owner, name)
if (get && set) {
const restored = readStoredField(store, key, undefined)
if (restored !== undefined) set.call(owner, restored)
const accessor = new StoredAccessor(owner, get, store, key)
accessors.push(accessor)
Object.defineProperty(owner, name, {
configurable: true,
enumerable: true,
get,
set: (value) => { set.call(owner, value); accessor.save() },
})
continue
}
let current = readStoredField(store, key, owner[name])
Object.defineProperty(owner, name, {
configurable: true,
enumerable: true,
get: () => current,
set: (value) => { current = value; writeStoredField(store, key, value) },
})
}
if (accessors.length) {
Object.defineProperty(owner, '__tachyonStoredChanged', {
configurable: true,
value: () => { for (const accessor of accessors) accessor.save() },
})
}
return owner
}
const persistScope = (id, properties) =>
`${id}:${properties?.persistId ?? properties?.['persist-id'] ?? location.pathname}`
const importAsset = (raw) => {
const url = new URL(raw, location.href)
const sameOrigin = url.origin === location.origin
const web = sameOrigin && url.pathname.startsWith('/.tachyon/services/')
const native = sameOrigin && url.pathname.includes('/WebBundle/tachyon-runtime/services/')
if (!web && !native) throw new TypeError('Tac service asset is not a generated same-origin URL.')
return import(url.href)
}
const createOwner = async (module, properties) => {
const controller = new AbortController()
try {
let owner
if (typeof module.__tachyon_create_owner__ !== 'function') {
const required = module.__tachyon_required_args__
owner = Array.isArray(required)
? new module.default(...required.map((name) => properties?.[name]))
: new module.default(properties)
} else {
owner = await module.__tachyon_create_owner__(properties, controller.signal, clientRequest)
}
Owners.managed.set(owner, controller)
Owners.modules.set(owner, module)
return owner
} catch (error) { controller.abort(); throw error }
}
const releaseOwner = (owner) => {
Owners.managed.get(owner)?.abort()
Owners.managed.delete(owner)
disposeNativeInstance(owner)
}
const disposeManagedOwner = (owner) => {
for (const entry of Services.byOwner.get(owner) || []) if (!entry.shared) disposeManagedOwner(entry.instance)
releaseOwner(owner)
}
class Services {
static catalog = ClientView.plan.services || {}
static singletons = new Map()
static resolved = new Map()
static creating = new Set()
static byOwner = new WeakMap()
static shared = new WeakSet()
static given(service, properties) {
if (!Array.isArray(service.required)) return properties
return Object.fromEntries(service.required.map((field) => [field, properties?.[field]]))
}
}
const requireProperties = (name, service, properties) => {
for (const field of service.required || []) {
if (properties?.[field] !== undefined && properties?.[field] !== null) continue
throw new TypeError(`Tac service '${name}' needs '${field}'; pass :${field}="..." to the component that uses it.`)
}
}
const createServiceInstance = async (name, service, properties, scope) => {
requireProperties(name, service, properties)
const given = Services.given(service, properties)
let instance
if (service.module) {
const module = await importAsset(service.module)
if (typeof module.default !== 'function') throw new TypeError(`Tac service '${name}' must default-export its class.`)
instance = await createOwner(module, given)
} else if (service.native && nativeHost()) {
instance = {}
} else {
throw new TypeError(`Tac service '${name}' has no browser module.`)
}
if (service.native && nativeHost()) await bindNativeInstance(instance, name, given)
Owners.stored.set(instance, service.stored || {})
attachRuntime(instance, scope)
if (service.native && nativeHost()) drainCompanionQueue()
for (const key of Object.keys(instance)) {
if (key === name || !Object.hasOwn(Services.catalog, key) || instance[key] !== undefined) continue
instance[key] = await serviceSingleton(key)
}
return instance
}
const registerSingleton = (name, instance) => {
Services.shared.add(instance)
for (const member of Runtime.graphs.get(instance) || [instance]) Services.shared.add(member)
Services.resolved.set(name, instance)
}
const disposeSharedInstance = (instance) => {
disposeRuntime(instance, true)
releaseOwner(instance)
}
const serviceSingleton = (name) => {
if (Services.creating.has(name)) throw new Error(`Tac services '${name}' depend on each other in a cycle.`)
if (Services.singletons.has(name)) return Services.singletons.get(name)
const service = Services.catalog[name]
if (!service) throw new TypeError(`Tac service '${name}' is not declared.`)
Services.creating.add(name)
const promise = createServiceInstance(name, service, {}, `service:${name}:${location.pathname}`)
.then((instance) => {
registerSingleton(name, instance)
return instance
})
.finally(() => Services.creating.delete(name))
Services.singletons.set(name, promise)
promise.catch(() => Services.singletons.delete(name))
return promise
}
const disposeSharedServices = () => {
for (const [name, instance] of Services.resolved) {
disposeSharedInstance(instance)
Services.resolved.delete(name)
Services.singletons.delete(name)
}
}
const composeOwner = (name, properties, entries) => {
const owner = { ...properties }
for (const entry of entries) {
Object.defineProperty(owner, entry.name, { value: entry.instance, enumerable: false, configurable: true, writable: true })
}
Services.byOwner.set(owner, entries)
attachRuntime(owner, persistScope(name, properties))
return owner
}
const componentOwner = async (node, properties, path) => {
Rendering.instanceKeys.add(path)
if (Components.instances.has(path)) return Components.instances.get(path)
const entries = []
for (const service of node.services || []) {
const shared = service.scope !== 'prototype'
const instance = shared
? await serviceSingleton(service.name)
: await createServiceInstance(service.name, service, properties, persistScope(`${node.name}.${service.name}`, properties))
entries.push({ name: service.name, service, instance, shared })
}
const owner = composeOwner(node.name, properties, entries)
Components.instances.set(path, owner)
Components.records.set(path, { node, properties, owner })
return owner
}
const snapshotState = (owner) => {
const state = {}
for (const [name, value] of Object.entries(owner || {})) {
if (typeof value !== 'function') state[name] = value
}
try { return structuredClone(state) } catch { return {} }
}
const isTextControl = (element) => element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement
const snapshotMutableDom = () => ({
active: document.activeElement?.id || null,
elements: [...document.querySelectorAll('[id]')].slice(0, 2048).map((element) => ({
id: element.id,
open: element instanceof HTMLDetailsElement ? element.open : undefined,
value: isTextControl(element) ? element.value : undefined,
checked: element instanceof HTMLInputElement ? element.checked : undefined,
scroll: element.scrollLeft || element.scrollTop ? [element.scrollLeft, element.scrollTop] : undefined,
})),
})
const restoreMutableDom = (state) => {
for (const saved of state.elements) {
const element = document.getElementById(saved.id)
if (!element) continue
if (saved.open !== undefined && element instanceof HTMLDetailsElement) element.open = saved.open
if (saved.value !== undefined && isTextControl(element)) element.value = saved.value
if (saved.checked !== undefined && element instanceof HTMLInputElement) element.checked = saved.checked
if (saved.scroll) element.scrollTo(saved.scroll[0], saved.scroll[1])
}
if (state.active) document.getElementById(state.active)?.focus({ preventScroll: true })
}
const carryHotState = async (stale, fresh) => {
const state = typeof stale.hotState === 'function' ? structuredClone(await stale.hotState()) : snapshotState(stale)
if (typeof stale.hotDispose === 'function') await stale.hotDispose()
if (typeof fresh.restoreHotState === 'function') await fresh.restoreHotState(state)
else Object.assign(fresh, state)
}
const hotService = (service, version) => {
const url = new URL(service.module, location.href)
url.searchParams.set('tachyon_hot', version || String(Date.now()))
return { ...service, module: url.href }
}
const serviceBoundary = (service) => {
const fileName = new URL(service.module, location.href).pathname.split('/').pop()
return `service:${fileName.replace(/\.js$/, '')}`
}
const replaceSharedService = async (entry, version) => {
const next = await createServiceInstance(entry.name, hotService(entry.service, version), {}, `service:${entry.name}:${location.pathname}`)
await carryHotState(entry.instance, next)
disposeSharedInstance(entry.instance)
registerSingleton(entry.name, next)
Services.singletons.set(entry.name, Promise.resolve(next))
return next
}
const replaceOwnedService = async (entry, record, version) => {
const scope = persistScope(`${record.node.name}.${entry.name}`, record.properties)
const instance = await createServiceInstance(entry.name, hotService(entry.service, version), record.properties, scope)
try { await carryHotState(entry.instance, instance) } catch (error) { disposeManagedOwner(instance); throw error }
return instance
}
const hotUpdate = async (boundaries, version) => {
const selected = new Set(boundaries || [])
const domState = snapshotMutableDom()
await clearStaticCache()
const replaced = new Map()
const affected = (service) => service.module && selected.has(serviceBoundary(service))
for (const [path, record] of Components.records) {
const entries = Services.byOwner.get(record.owner) || []
if (!entries.some((entry) => affected(entry.service))) continue
const fresh = []
for (const entry of entries) {
let instance = entry.instance
if (affected(entry.service) && entry.shared) {
if (!replaced.has(entry.name)) replaced.set(entry.name, await replaceSharedService(entry, version))
instance = replaced.get(entry.name)
} else if (affected(entry.service)) {
instance = await replaceOwnedService(entry, record, version)
}
fresh.push({ ...entry, instance })
}
const owner = composeOwner(record.node.name, record.properties, fresh)
disposeOwner(record.owner)
cancelMount(path)
record.owner = owner
record.host = undefined
Components.instances.set(path, owner)
}
await render()
while (Components.activating.size) await Promise.allSettled([...Components.activating])
if (Rendering.current) await Rendering.current.catch(() => {})
restoreMutableDom(domState)
}
const trackActivation = (promise) => {
Components.activating.add(promise)
promise.finally(() => Components.activating.delete(promise)).catch(() => {})
return promise
}
const activateOnInteraction = (host, pending, activate) => {
const stopListening = () => {
host.removeEventListener('pointerdown', listener, true)
host.removeEventListener('keydown', listener, true)
}
const listener = async (event) => {
if (event.cancelable) event.preventDefault()
event.stopImmediatePropagation()
stopListening()
const target = event.target
if (await activate()) {
target.dispatchEvent(new Event(event.type, { bubbles: true, cancelable: true }))
}
}
host.addEventListener('pointerdown', listener, true)
host.addEventListener('keydown', listener, true)
pending.cleanup = stopListening
}
const scheduleActivation = (host, policy, pending, activate) => {
if (policy === 'idle' && globalThis.requestIdleCallback) {
const id = requestIdleCallback(activate)
pending.cleanup = () => globalThis.cancelIdleCallback?.(id)
} else if (policy === 'idle') {
const id = setTimeout(activate, 1)
pending.cleanup = () => clearTimeout(id)
} else if (policy === 'visible' && globalThis.IntersectionObserver) {
const observer = new IntersectionObserver((entries) => {
if (entries.some((entry) => entry.isIntersecting)) { observer.disconnect(); void activate() }
}, { rootMargin: '100px' })
pending.cleanup = () => observer.disconnect()
observer.observe(host)
} else if (policy === 'interaction') {
activateOnInteraction(host, pending, activate)
} else queueMicrotask(activate)
}
const mountComponent = (host, owner, policy, key) => {
const schedules = Components.mountSchedules
const activations = Components.activations
if (activations.has(owner)) return
const previous = schedules.get(key)
if (previous?.host === host && previous.owner === owner && previous.policy === policy) return
cancelMount(key)
if (!policy || policy === 'never') return
const pending = { host, owner, policy, cancelled: false, cleanup: () => {}, cancel: () => {} }
const release = () => {
pending.cleanup()
pending.cleanup = () => {}
if (schedules.get(key) === pending) schedules.delete(key)
}
pending.cancel = () => { pending.cancelled = true; release() }
schedules.set(key, pending)
const stillOwned = () => host.isConnected && Components.records.get(key)?.owner === owner
const activate = () => trackActivation(activateNow())
const activateNow = async () => {
if (pending.cancelled || !stillOwned()) { pending.cancel(); return false }
if (activations.has(owner)) { release(); return false }
activations.set(owner, 'initializing')
release()
try {
attachRuntime(owner, key, host)
await initializeRuntime(owner)
if (!stillOwned()) return false
activations.set(owner, 'active')
host.dataset.tachyonActive = 'true'
host.removeAttribute('data-tachyon-mount-error')
trackActivation(render())
return true
} catch {
activations.set(owner, 'failed')
disposeOwner(owner)
if (!stillOwned()) return false
host.dataset.tachyonActive = 'false'
host.dataset.tachyonMountError = 'activation_failed'
return false
}
}
scheduleActivation(host, policy, pending, activate)
}
const renderText = async (parts, environment) => {
let value = ''
for (const part of parts || []) {
value += 'expression' in part ? display(await evaluate(part.expression, environment)) : part.value
}
return document.createTextNode(value)
}
const recordElement = (element, node, environment, path) => {
const locals = []
for (const name in environment.locals) locals.push([name, environment.locals[name]])
Elements.records.set(element, { node, owner: environment.owner, path,
locals, children: [...element.childNodes],
attributes: new Map(element.getAttributeNames().map((name) => [name, element.getAttribute(name)])) })
Rendering.elements.set(path, element)
return element
}
const sameLocals = (previous, next) => previous.length === next.length
&& previous.every(([name, value], index) => next[index][0] === name && Object.is(next[index][1], value))
const retainedPair = (old, freshChild) => {
const previous = Elements.records.get(old)
const fresh = previous && Rendering.elements.get(previous.path)
const next = fresh && Elements.records.get(fresh)
if (!next || previous.node !== next.node || previous.owner !== next.owner) return null
if (old.tagName !== fresh.tagName || old.namespaceURI !== fresh.namespaceURI || !sameLocals(previous.locals, next.locals)) return null
if (freshChild && freshChild.parentNode !== fresh) return null
return { old, fresh, previous }
}
const focusChain = (active, fragment, container) => {
const chain = []
let old = active
let freshChild = null
while (old && old !== container) {
if (!Elements.records.has(old) && chain.length) {
old = old.parentNode
continue
}
const pair = retainedPair(old, freshChild)
if (!pair) return []
chain.unshift(pair)
freshChild = pair.fresh
old = old.parentNode
}
if (old !== container || chain[0]?.fresh.parentNode !== fragment) return []
return chain
}
const ownedSlots = (parent) => [...parent.querySelectorAll('slot')].filter((slot) => {
for (let node = slot.parentElement; node && node !== parent; node = node.parentElement) {
if (Elements.records.has(node)) return false
}
return true
})
const distributionGroups = (parent, source, focusedChild) => {
const slots = ownedSlots(parent)
const groups = new Map(slots.map((slot) => [slot.getAttribute('name') || '', { slot, children: [] }]))
if (!slots.includes(focusedChild.parentNode) || groups.size !== slots.length) return null
for (const child of [...source.childNodes]) {
const name = child.nodeType === Node.ELEMENT_NODE ? child.getAttribute('slot') || '' : ''
const group = groups.get(name)
if (!group) return null
group.children.push(child)
}
return groups
}
const changesObservedAttribute = ({ old, fresh, previous }) => {
if (!old.localName.includes('-')) return false
try {
const observed = old.constructor.observedAttributes || []
if (!Array.isArray(observed) || observed.length > 256) return true
const attributes = Elements.records.get(fresh).attributes
return observed.some((name) => previous.attributes.get(name) !== attributes.get(name))
} catch { return true }
}
const preflightFocus = (chain, fragment, container) => {
const distributions = new Map()
for (let depth = 0; depth < chain.length; depth += 1) {
const pair = chain[depth]
if (changesObservedAttribute(pair)) return null
const parent = depth ? chain[depth - 1].old : container
if (pair.old.parentNode === parent) continue
const source = depth ? chain[depth - 1].fresh : fragment
const groups = depth ? distributionGroups(parent, source, pair.old) : null
if (!groups) return null
distributions.set(pair, groups)
}
return distributions
}
const commitAndRefocus = (authored, active, replacement) => {
const selection = [active.selectionStart, active.selectionEnd, active.selectionDirection]
authored.commit()
replacement.focus({ preventScroll: true })
if (selection[0] !== null && selection[0] !== undefined) {
try { replacement.setSelectionRange(...selection) } catch {}
}
}
const syncAuthoredAttributes = (element, previous, next) => {
for (const [name] of previous.attributes) {
if (!next.attributes.has(name)) Elements.removeAttribute(element, name)
}
for (const [name, value] of next.attributes) {
if (previous.attributes.get(name) !== value) Elements.setAttribute(element, name, value)
}
}
const changedDuringRender = (element, active, editingAtStart) => element === active
&& editingAtStart?.element === element && editingAtStart.value !== element.value
const syncControlValue = (element, fresh, next, active, editingAtStart) => {
if (!next.attributes.has('value') || !('value' in element)) return
if (changedDuringRender(element, active, editingAtStart) || element.value === fresh.value) return
try { element.value = fresh.value } catch {}
}
const syncControlFlags = (element, fresh, previous, next) => {
for (const name of ['checked', 'disabled', 'selected']) {
if (previous.attributes.has(name) || next.attributes.has(name)) element[name] = fresh[name]
}
}
const transferEventListeners = (element, fresh) => {
for (const { type, listener } of Elements.listeners.get(element) || []) element.removeEventListener(type, listener)
const listeners = Elements.listeners.get(fresh) || []
for (const { type, listener } of listeners) element.addEventListener(type, listener)
Elements.listeners.set(element, listeners)
}
class FocusedRootUpdate {
constructor(chain, distributions, active, editingAtStart) {
Object.assign(this, { chain, distributions, active, editingAtStart })
}
syncElement(element, fresh) {
const previous = Elements.records.get(element)
const next = Elements.records.get(fresh)
syncAuthoredAttributes(element, previous, next)
syncControlValue(element, fresh, next, this.active, this.editingAtStart)
syncControlFlags(element, fresh, previous, next)
transferEventListeners(element, fresh)
Elements.records.set(element, next)
if (fresh.tachyonComponent) element.tachyonComponent = fresh.tachyonComponent
}
commit(parent, source, depth) {
const pair = this.chain[depth]
if (!pair) {
if (!parent.matches('input, textarea, [contenteditable]')) parent.replaceChildren(...source.childNodes)
return
}
if (pair.old.parentNode !== parent) {
this.commitSlots(pair, depth)
return
}
this.commitSiblings(parent, source, pair, depth)
}
commitSlots(pair, depth) {
const owner = this.chain[depth - 1]
for (const { slot, children } of this.distributions.get(pair).values()) {
if (slot === pair.old.parentNode) {
const content = document.createDocumentFragment()
content.append(...children)
this.commit(slot, content, depth)
} else if (children.length || owner.previous.children.some((child) => child.parentNode === slot)) {
slot.replaceChildren(...children)
}
}
}
commitSiblings(parent, source, pair, depth) {
for (const child of [...parent.childNodes]) if (child !== pair.old) child.remove()
let before = true
for (const child of [...source.childNodes]) {
if (child === pair.fresh) {
this.syncElement(pair.old, pair.fresh)
this.commit(pair.old, pair.fresh, depth + 1)
before = false
} else if (before) parent.insertBefore(child, pair.old)
else parent.append(child)
}
}
finalize() {
const retained = new Map(this.chain.map((pair) => [pair.fresh, pair.old]))
for (const { old, fresh } of this.chain) {
const record = Elements.records.get(old)
record.children = Elements.records.get(fresh).children.map((child) => retained.get(child) || child)
}
Rendering.mounts = Rendering.mounts.map(([host, ...arguments_]) => [retained.get(host) || host, ...arguments_])
}
}
const focusedElement = () => {
let active = document.activeElement
while (active?.shadowRoot?.activeElement) active = active.shadowRoot.activeElement
return active
}
const commitRoot = (container, fragment, editingAtStart) => {
const active = container.getRootNode().activeElement
const chain = focusChain(active, fragment, container)
const authored = new AuthoredRootUpdate(container, fragment, active, editingAtStart).prepare()
if (!chain.length || authored.retains(active)) { authored.commit(); return }
const distributions = preflightFocus(chain, fragment, container)
if (!distributions || authored.refuses(chain)) { commitAndRefocus(authored, active, chain.at(-1).fresh); return }
const update = new FocusedRootUpdate(chain, distributions, active, editingAtStart)
update.commit(container, fragment, 0)
update.finalize()
}
const retainComponentRoots = () => {
const retained = new Map()
Rendering.mounts = Rendering.mounts.map(([fresh, owner, policy, key]) => {
const record = Components.records.get(key)
const previous = record?.host
if (!record || !previous || previous === fresh) {
if (record) record.host = fresh
return [fresh, owner, policy, key]
}
syncAuthoredAttributes(previous, Elements.records.get(previous), Elements.records.get(fresh))
const active = focusedElement()
const editing = active && fresh.contains(active) ? active : null
const selection = editing && typeof editing.selectionStart === 'number'
? [editing.selectionStart, editing.selectionEnd, editing.selectionDirection] : null
new AuthoredRootUpdate(previous, fresh, active, null).prepare().commit()
Elements.records.set(previous, { ...Elements.records.get(fresh), children: [...previous.childNodes] })
previous.tachyonComponent = fresh.tachyonComponent
fresh.replaceWith(previous)
if (editing && document.activeElement !== editing) {
editing.focus({ preventScroll: true })
if (selection && typeof editing.setSelectionRange === 'function') {
try { editing.setSelectionRange(selection[0], selection[1], selection[2] || 'none') } catch {}
}
}
retained.set(fresh, previous)
return [previous, owner, policy, key]
})
for (const [path, fresh] of Rendering.elements) {
const element = retained.get(fresh) || fresh
Rendering.elements.set(path, element)
const record = Elements.records.get(element)
if (record) record.children = record.children.map(child => retained.get(child) || child)
}
}
const renderComponent = async (node, environment, path) => {
const properties = {}
for (const property of node.properties || []) {
if (property.event) continue
properties[property.name] = 'expression' in property
? await evaluate(property.expression, environment)
: property.value
}
const componentKey = `${path}:${node.name}`
const owner = await componentOwner(node, properties, componentKey)
for (const [name, value] of Object.entries(properties)) {
if (node.exported || !(name in owner)) owner[name] = value
}
const host = document.createElement('tachyon-component')
host.dataset.tachyonComponent = node.name
host.dataset.tachyonMount = node.mount || 'none'
if (node.scope) Elements.markScope(host, node.name)
host.append(await renderNodes(node.template || [], ownedEnvironment(owner), `${path}.template`, {
namespace: Elements.rootContext.namespace,
slot: { nodes: node.slot || [], named: node.slots || {}, environment },
}))
host.tachyonComponent = { instance: owner, refresh: render }
Rendering.mounts.push([host, owner, node.mount, componentKey])
return recordElement(host, node, environment, path)
}
const countIteration = () => {
if (++Rendering.iterationCount > 100000) throw new RangeError('Tac client view exceeds 100,000 iterations.')
}
const renderIteration = async (node, environment, path, context) => {
const fragment = document.createDocumentFragment()
const values = await evaluate(node.iterable, environment)
if (!Array.isArray(values)) return fragment
if (values.length > 10000) throw new RangeError('Tac iteration exceeds 10,000 items.')
for (let index = 0; index < values.length; index += 1) {
countIteration()
const local = childEnvironment(environment, { [node.binding]: values[index], $index: index })
fragment.append(await renderNodes(node.children || [], local, `${path}.${index}`, context))
}
return fragment
}
const countedLoopContinues = (comparison, counter, to) => {
if (comparison === 'lt') return counter < to
if (comparison === 'le') return counter <= to
if (comparison === 'gt') return counter > to
return counter >= to
}
const renderCounted = async (node, environment, path, context) => {
const fragment = document.createDocumentFragment()
const from = Number(await evaluate(node.from, environment))
const to = Number(await evaluate(node.to, environment))
const magnitude = Number(await evaluate(node.step, environment))
const ascending = node.comparison === 'lt' || node.comparison === 'le'
const step = ascending ? magnitude : -magnitude
if (!Number.isFinite(from) || !Number.isFinite(to) || !Number.isFinite(step) || magnitude <= 0) return fragment
let index = 0
for (let counter = from; countedLoopContinues(node.comparison, counter, to); counter += step) {
countIteration()
if (index >= 10000) throw new RangeError('Tac iteration exceeds 10,000 items.')
const local = childEnvironment(environment, { [node.binding]: counter, $index: index })
fragment.append(await renderNodes(node.children || [], local, `${path}.${index}`, context))
index += 1
if (counter + step === counter) throw new RangeError('Tac iteration step makes no progress.')
}
return fragment
}
const renderSwitch = async (node, environment, path, context) => {
const value = await evaluate(node.value, environment)
let fallback
for (const candidate of node.children || []) {
if (candidate.k !== 'case') continue
if (!candidate.when) fallback = candidate
else if (value === await evaluate(candidate.when, environment)) {
return renderNodes(candidate.children || [], environment, `${path}.case`, context)
}
}
if (!fallback) return document.createDocumentFragment()
return renderNodes(fallback.children || [], environment, `${path}.default`, context)
}
const renderNode = async (node, environment, path, context) => {
Rendering.nodeCount += 1
if (Rendering.nodeCount > 100000) throw new RangeError('Tac client view exceeds 100,000 nodes.')
if (node.k === 'text') return renderText(node.parts, environment)
if (node.k === 'comment') return document.createComment(node.value || '')
if (node.k === 'slot') {
const slot = context.slot
const nodes = node.name ? slot?.named[node.name] : slot?.nodes
return nodes ? renderNodes(nodes, slot.environment, `${path}.slot`, { slot: null, namespace: context.namespace }) : document.createDocumentFragment()
}
if (node.k === 'iteration') return renderIteration(node, environment, path, context)
if (node.k === 'counted') return renderCounted(node, environment, path, context)
if (node.k === 'switch') return renderSwitch(node, environment, path, context)
if (node.k === 'component') return renderComponent(node, environment, path)
if (node.k !== 'element') return document.createDocumentFragment()
const element = Elements.create(node.tag, context.namespace)
await applyAttributes(element, node.attributes, environment)
if (!node.void) {
const children = node.children || []
const childContext = Elements.childContext(element, context)
element.append(await renderNodes(children, environment, `${path}.children`, childContext))
}
return recordElement(element, node, environment, path)
}
const renderNodes = async (nodes, environment, path, context = Elements.rootContext) => {
const fragment = document.createDocumentFragment()
for (let index = 0; index < nodes.length; index += 1) {
const node = nodes[index]
if (node.k === 'conditional') {
if (node.branch !== 'if') continue
let cursor = index
let selected
while (cursor < nodes.length) {
const candidate = nodes[cursor]
if (cursor > index && (candidate.k === 'comment'
|| (candidate.k === 'text' && candidate.parts?.every((part) => 'value' in part && !part.value.trim())))) {
cursor += 1
continue
}
if (candidate.k !== 'conditional') break
if (!candidate.condition || await evaluate(candidate.condition, environment)) {
selected = candidate
cursor += 1
break
}
cursor += 1
}
if (selected) fragment.append(await renderNodes(selected.children || [], environment, `${path}.${index}.branch`, context))
index = cursor - 1
continue
}
fragment.append(await renderNode(node, environment, `${path}.${index}`, context))
}
return fragment
}
class TacElements {
static connected = new Set()
static created = 0
static stylesheet = null
static define(definition) {
for (const [name, service] of Object.entries(definition.services || {})) {
if (!Object.hasOwn(Services.catalog, name)) Services.catalog[name] = service
}
if (!customElements.get(definition.node.name)) customElements.define(definition.node.name, TacElements.elementClass(definition))
}
static elementClass(definition) {
const attributes = new Map(definition.props.map((name) => [TacElements.kebab(name), name]))
const element = class extends HTMLElement {
static observedAttributes = [...attributes.keys()]
constructor() {
super()
this.attachShadow({ mode: 'open' })
this._tacDefinition = definition
this._tacPath = `element.${TacElements.created += 1}`
this._tacNode = { ...definition.node, properties: [] }
this._tacProperties = Object.create(null)
for (const name of definition.props) TacElements.upgradeProperty(this, name)
}
connectedCallback() { TacElements.connected.add(this); TacElements.refresh() }
disconnectedCallback() { TacElements.connected.delete(this); TacElements.refresh() }
attributeChangedCallback(attribute, _previous, value) { TacElements.change(this, attributes.get(attribute), value) }
}
for (const name of definition.props) {
Object.defineProperty(element.prototype, name, {
configurable: true,
enumerable: true,
get() { return this._tacProperties[name] },
set(value) { TacElements.change(this, name, value) },
})
}
return element
}
static upgradeProperty(element, name) {
if (!Object.hasOwn(element, name)) return
const value = element[name]
delete element[name]
element[name] = value
}
static kebab(name) {
return name.replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase()
}
static change(element, name, value) {
element._tacProperties[name] = value
element._tacNode.properties = Object.entries(element._tacProperties).map(([key, given]) => ({ name: key, value: given }))
if (element.isConnected) TacElements.refresh()
}
static refresh() {
void render().catch(signalFailure)
}
static loadStylesheet() {
TacElements.stylesheet ??= fetch(new URL('components.css', import.meta.url))
.then((response) => response.ok ? response.text() : '')
.then((text) => {
const sheet = new CSSStyleSheet()
sheet.replaceSync(text)
return sheet
})
return TacElements.stylesheet
}
static async renderInto(element, environment, editingAtStart) {
try {
const root = element.shadowRoot
if (element._tacDefinition.styles && !root.adoptedStyleSheets.length) root.adoptedStyleSheets = [await TacElements.loadStylesheet()]
commitRoot(root, await renderNodes([element._tacNode], environment, element._tacPath), editingAtStart)
delete element.dataset.tachyonRenderError
} catch (error) {
element.dataset.tachyonRenderError = 'render_failed'
globalThis.reportError?.(error)
}
}
static announce(view, name, value) {
const root = view?.getRootNode?.()
if (!(root instanceof ShadowRoot) || !TacElements.connected.has(root.host)) return
const detail = JSON.parse(JSON.stringify(value))
view.dispatchEvent(new CustomEvent(TacElements.kebab(name), { detail, bubbles: true, composed: true }))
}
}
const renderDocument = async (environment) => {
Rendering.nodeCount = 0
Rendering.iterationCount = 0
Rendering.instanceKeys = new Set()
Rendering.elements = new Map()
Rendering.mounts = []
const focused = focusedElement()
const editingAtStart = { element: focused, value: focused?.value }
if (ClientView.page) commitRoot(document.body, await renderNodes(ClientView.plan.nodes || [], environment, 'body'), editingAtStart)
for (const element of TacElements.connected) await TacElements.renderInto(element, environment, editingAtStart)
retainComponentRoots()
for (const arguments_ of Rendering.mounts) mountComponent(...arguments_)
for (const [key, record] of Components.records) {
if (Rendering.instanceKeys.has(key)) continue
cancelMount(key)
disposeOwner(record.owner)
Components.records.delete(key)
Components.instances.delete(key)
}
}
const render = async () => {
if (Rendering.current) { Rendering.renderAgain = true; return Rendering.current }
Rendering.current = (async () => {
do {
Rendering.renderAgain = false
await NativeState.writes
const scroll = { x: scrollX, y: scrollY }
await renderDocument(ownedEnvironment(Page.owner))
scrollTo(scroll.x, scroll.y)
} while (Rendering.renderAgain)
markDocument('tachyonRendered', 'client')
delete document.documentElement.dataset.tachyonRenderError
})().catch((error) => {
markDocument('tachyonRenderError', 'render_failed')
throw error
}).finally(() => { Rendering.current = null })
return Rendering.current
}
const signalFailure = () => {
markDocument('tachyonSignalError', 'listener_failed')
}
class Runtime {
static events = new TacApplicationEvents({
changed: () => { void render().catch(signalFailure) },
failed: signalFailure,
calling: (owner, work) => TacWebApis.calling(owner, work),
announced: (view, name, value) => TacElements.announce(view, name, value),
})
static graphs = new WeakMap()
static watching = TacWebApis.watch({
publish: (name, value) => {
try { Runtime.events.publish(name, value, { retain: true }) } catch { signalFailure() }
},
send: (owner, name, value) => {
try { Runtime.events.send(owner, name, value) } catch { signalFailure() }
},
})
}
const ownerResources = (element, signal) => ({
view: element, lifecycle: null, applicationEventPublisher: null,
resourceLoader: Object.freeze({ precache: precacheAssets, clearAssetCache: clearStaticCache }),
taskScheduler: Object.freeze({ render }),
clock: TacWebApis.clock,
secureRandom: TacWebApis.secureRandom,
connectivity: TacWebApis.connectivity,
pageVisibility: TacWebApis.pageVisibility,
preferences: TacWebApis.preferences,
preferenceStrings: TacWebApis.preferenceStrings,
documentTitle: TacDocumentTitle.owned(signal),
clipboard: TacWebApis.clipboard,
share: TacWebApis.share,
notifications: TacWebApis.notifications,
geolocation: TacWebApis.geolocation,
windowControls: TacWebApis.windowControls,
filePicker: TacNativeCapabilities.filePicker(signal),
})
const attachRuntime = (owner, scope, element) => {
if (!owner || typeof owner !== 'object') return owner
const composed = Services.byOwner.get(owner)
if (composed) {
const graph = []
for (const entry of composed) {
attachRuntime(entry.instance, entry.shared ? `service:${entry.name}:${location.pathname}` : scope, element)
graph.push(...(Runtime.graphs.get(entry.instance) || [entry.instance]))
}
Runtime.graphs.set(owner, graph)
return bindPersistentFields(owner, scope)
}
const module = Owners.modules.get(owner)
const graph = Runtime.graphs.get(owner) || module?.__tachyon_event_owners__?.(owner) || [owner]
Runtime.graphs.set(owner, graph)
const resources = ownerResources(element, Owners.managed.get(owner)?.signal)
for (const instance of graph) {
const metadata = module?.__tachyon_event_plan__?.types?.find((type) => type.name === instance.constructor?.name) || {}
Runtime.events.attach(instance, metadata, resources)
}
return bindPersistentFields(owner, scope)
}
const initializeRuntime = async (owner) => {
for (const instance of Runtime.graphs.get(owner) || [owner]) await Runtime.events.initialize(instance)
}
const disposeRuntime = (owner, shared = false) => {
for (const instance of [...(Runtime.graphs.get(owner) || [owner])].reverse()) {
if (!shared && Services.shared.has(instance)) continue
Runtime.events.dispose(instance)
}
}
const disposeOwner = (owner) => {
disposeRuntime(owner)
disposeManagedOwner(owner)
}
const nativeHost = () => typeof globalThis.__tachyonNativeHostCall === 'function'
const nativeCall = async (request, ownerId, service) => {
const payload = JSON.stringify({ ...request, route: ClientView.plan.route, service, owner: ownerId })
const limit = request.op === 'init' ? NativeState.startUpRequestLimit : NativeState.memberRequestLimit
if (payload.length > limit || new TextEncoder().encode(payload).byteLength > limit) throw new RangeError(NativeState.tooLarge(request, service, payload, limit))
let timer
try {
const raw = await Promise.race([
Promise.resolve(globalThis.__tachyonNativeHostCall('companion.invoke', payload)),
new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('Tac native call timed out.')), 10000) }),
])
if (typeof raw === 'string' && (raw.length > 65536 || new TextEncoder().encode(raw).byteLength > 65536)) throw new RangeError('Tac native response exceeds 64 KiB.')
const answer = typeof raw === 'string' ? JSON.parse(raw) : raw
if (!answer || answer.error || answer.ok === false) throw new Error('Tac native call failed.')
return answer.value
} finally { clearTimeout(timer) }
}
const nativeMember = (name) => typeof name === 'string'
&& /^[A-Za-z_$][A-Za-z0-9_$]{0,127}$/.test(name)
&& !['__proto__', 'prototype', 'constructor', 'tac'].includes(name)
const bindNativeFields = (owner, names, call) => {
const state = new Map()
const revisions = new Map()
const pending = new Map()
const readField = async (name, revision = revisions.get(name) ?? 0) => {
const value = await call({ op: 'get', name })
if ((revisions.get(name) ?? 0) === revision) state.set(name, value)
}
const refresh = async () => {
for (const name of names) {
if (!pending.has(name)) await readField(name)
}
}
for (const name of names) {
Object.defineProperty(owner, name, {
configurable: true,
enumerable: true,
get: () => state.get(name),
set: (value) => {
if (NativeState.writeCount >= 128) throw new RangeError('Tac native assignment queue exceeds 128 writes.')
NativeState.writeCount += 1
const revision = (revisions.get(name) ?? 0) + 1
revisions.set(name, revision)
pending.set(name, revision)
state.set(name, value)
NativeState.writes = NativeState.writes.catch(() => {}).then(async () => {
await call({ op: 'set', name, value })
await readField(name, revision)
}).finally(() => {
NativeState.writeCount -= 1
if (pending.get(name) === revision) pending.delete(name)
})
void NativeState.writes.catch(() => { markDocument('tachyonNativeError', 'assignment_failed') })
},
})
}
return refresh
}
const drainCompanionQueue = () => {
const queue = globalThis.__tachyonCompanionQueue
const queued = Array.isArray(queue) ? queue.splice(0).slice(-128) : []
for (const signal of queued) globalThis.__tachyonCompanionPublish?.(signal)
}
const bindNativeInstance = async (owner, service, properties) => {
const ownerId = crypto.randomUUID()
const record = { ownerId, service, disposed: false }
const call = (request) => {
if (record.disposed && request.op !== 'dispose') throw new Error('Tac native owner disposed.')
return nativeCall(request, ownerId, service)
}
let props = {}
try { props = JSON.parse(JSON.stringify(properties ?? {})) ?? {} } catch {}
const declared = await call({ op: 'init', props })
const fields = (Array.isArray(declared?.fields) ? declared.fields : []).slice(0, 128).filter(nativeMember)
const refreshNativeFields = bindNativeFields(owner, fields, call)
await refreshNativeFields()
for (const name of (Array.isArray(declared?.methods) ? declared.methods : []).slice(0, 128)) {
if (!nativeMember(name)) continue
const method = async (...args) => {
await NativeState.writes
const answer = await call({ op: 'call', name, args })
await refreshNativeFields()
return answer
}
NativeState.methods.add(method)
owner[name] = method
}
NativeState.owners.set(owner, record)
}
const disposeNativeInstance = (owner) => {
const record = NativeState.owners.get(owner)
if (!record || record.disposed) return
record.disposed = true
NativeState.owners.delete(owner)
void nativeCall({ op: 'dispose' }, record.ownerId, record.service).catch(signalFailure)
}
if (nativeHost()) {
globalThis.__tachyonCompanionPublish = (signal) => {
if (signal?.route !== ClientView.plan.route) return
const live = [...NativeState.owners.values()].some((record) => !record.disposed && record.ownerId === signal?.owner && record.service === signal?.service)
if (!live) return
try { Runtime.events.publish(signal.name, signal.value, { retain: signal.retain === true }) } catch { signalFailure() }
}
drainCompanionQueue()
}
addEventListener('pagehide', (event) => {
if (event.persisted) return
for (const record of Components.records.values()) disposeOwner(record.owner)
disposeSharedServices()
})
Object.defineProperty(globalThis, '__tachyonTac', {
configurable: true,
value: Object.freeze({ render, hotUpdate, instance: Page.owner, instances: Components.instances, service: (name) => Services.resolved.get(name) }),
})
globalThis.__tc_rerender = render
addEventListener('tachyon:rerender', render)
await TacStorage.ready()
await render()
await render()
const whenIdle = (task) => globalThis.requestIdleCallback ? requestIdleCallback(task) : setTimeout(task, 1)
whenIdle(() => {
void precacheAssets(planAssets(ClientView.plan.nodes).map((asset) => new URL(asset, location.href).href)).catch(() => {
markDocument('tachyonCacheError', 'precache_failed')
})
})
export { TacElements }
