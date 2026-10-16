/** Public boundary for the retained FYLO 26.33.02 Node adapter.
 * The adapter transports documents through NDJSON; callers must validate their
 * domain DTOs after reading. Unqualified protocol result shapes stay unknown.
 */
export type Document = Record<string, unknown>
export type DocumentRecord = Record<string, Document>
export type Query = Record<string, unknown>
export type AccessInput = { uid?: number; groups?: number[]; gid?: number; mode?: number }
export interface AccessOperation<Result> extends Promise<Result> {
  /** Selects the access identity before the operation's first microtask. */
  as(access: AccessInput): AccessOperation<Result>
}
export interface Options {
  binary?: string
  env?: string | Record<string, string | number | boolean | null | undefined>
  worm?: boolean
  exclusiveRoot?: boolean
  strictSchema?: boolean
  shardWidth?: number
  bridgeAddress?: string
  bridgeSecret?: string
  maxRequestBytes?: number
  maxResponseBytes?: number
}
export interface Identity {
  runtimeVersion: string
  protocolVersion: number
  capabilities: { handshake: boolean; [capability: string]: unknown }
  machine?: { maxRequestBytes: number; maxResponseBytes: number; [field: string]: unknown }
}
export interface FileOptions { key?: string; maxBytes?: number; meta?: object }
export interface InlineFileData {
  encoding: 'base64'
  data: string
  contentLength: number
  checksumSHA256: string
}
export interface PageOptions { cursor?: string; limit?: number }
export interface DocumentPage { items: DocumentRecord; nextCursor: string | null }
export type SqlValue = string | number | boolean | bigint | Date | null | undefined

export class Fylo {
  constructor(root: string, options?: Options)
  readonly root: string
  readonly ready: Promise<Identity>
  readonly identity?: Identity
  readonly maxRequestBytes: number
  readonly maxResponseBytes: number
  handshake(): Promise<Identity>
  close(): Promise<void>
  request(operation: Record<string, unknown>): Promise<unknown>
  queuePublish(topic: string, payload: unknown, options?: { delayMs?: number; idempotencyKey?: string }): Promise<unknown>
  queueClaim(topic: string, group: string, options?: { maxMessages?: number; visibilityTimeoutMs?: number; maxAttempts?: number }): Promise<unknown>
  queueAck(topic: string, group: string, id: string, receipt: string): Promise<unknown>
  queueNack(topic: string, group: string, id: string, receipt: string, options?: { delayMs?: number; reason?: string }): Promise<unknown>
  queueExtend(topic: string, group: string, id: string, receipt: string, visibilityTimeoutMs: number): Promise<unknown>
  queueStats(topic: string, group: string): Promise<unknown>
  queueDeadLetters(topic: string, group: string, limit?: number): Promise<unknown>
  createCollection(collection: string, kind?: 'document' | 'file'): Promise<unknown>
  dropCollection(collection: string): Promise<unknown>
  inspectCollection(collection: string): Promise<unknown>
  rebuildCollection(collection: string): Promise<unknown>
  putData(collection: string, data: object, meta?: object): AccessOperation<string>
  putFile(collection: string, filePath: string, options?: FileOptions): AccessOperation<string>
  getFileBytes(collection: string, id: string): AccessOperation<Uint8Array>
  batchPutData(collection: string, batch: object[]): AccessOperation<unknown>
  getDoc(collection: string, id: string): AccessOperation<DocumentRecord>
  getFileData(collection: string, id: string, path?: string): AccessOperation<InlineFileData>
  getMeta(collection: string, id: string): AccessOperation<Document>
  setMeta(collection: string, id: string, meta: object): AccessOperation<unknown>
  getLatest(collection: string, id: string, onlyId?: false): AccessOperation<DocumentRecord>
  getLatest(collection: string, id: string, onlyId: true): AccessOperation<string | null>
  getLatest(collection: string, id: string, onlyId: boolean): AccessOperation<DocumentRecord | string | null>
  patchDoc(collection: string, id: string, newDocument: object, oldDocument?: object): AccessOperation<string>
  patchDocs(collection: string, update: Query): AccessOperation<number>
  delDoc(collection: string, id: string): AccessOperation<{ deleted: boolean }>
  delDocs(collection: string, criteria: Query): AccessOperation<number>
  restoreDoc(collection: string, id: string): AccessOperation<unknown>
  findDocs(collection: string, query: Query): AccessOperation<DocumentRecord>
  findDeletedDocs(collection: string, query?: Query): AccessOperation<DocumentRecord>
  findDocsPage(collection: string, query: Query, page?: PageOptions): AccessOperation<DocumentPage>
  findDeletedDocsPage(collection: string, query?: Query, page?: PageOptions): AccessOperation<DocumentPage>
  joinDocs(join: Query): AccessOperation<DocumentRecord>
  executeSQL(statement: string, access?: AccessInput): Promise<unknown>
  sql(strings: TemplateStringsArray, ...values: SqlValue[]): AccessOperation<unknown>
  importBulkData(collection: string, url: string, limitOrOptions?: number | Document): Promise<unknown>
  collection(name: string): Collection
}
export interface Collection {
  create(kind?: 'document' | 'file'): Promise<unknown>
  drop(): Promise<unknown>
  inspect(): Promise<unknown>
  rebuild(): Promise<unknown>
  put(data: object, meta?: object): AccessOperation<string>
  putFile(filePath: string, options?: FileOptions): AccessOperation<string>
  bytes(id: string): AccessOperation<Uint8Array>
  get(id: string): AccessOperation<DocumentRecord>
  getMeta(id: string): AccessOperation<Document>
  setMeta(id: string, meta: object): AccessOperation<unknown>
  latest(id: string, onlyId?: boolean): AccessOperation<DocumentRecord | string | null>
  patch(id: string, newDocument: object, oldDocument?: object): AccessOperation<string>
  delete(id: string): AccessOperation<{ deleted: boolean }>
  restore(id: string): AccessOperation<unknown>
  find(query: Query): AccessOperation<DocumentRecord>
  findPage(query: Query, page?: PageOptions): AccessOperation<DocumentPage>
}
