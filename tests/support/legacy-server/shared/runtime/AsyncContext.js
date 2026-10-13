import { AsyncLocalStorage } from 'node:async_hooks'

/** Keeps the caller's async context (such as FYLO access) for work queued later. */
export class AsyncContext {
  /**
   * @template {(...args: any[]) => any} T
   * @param {T} task
   * @returns {T}
   */
  static bind(task) {
    return AsyncLocalStorage.bind(task)
  }
}
