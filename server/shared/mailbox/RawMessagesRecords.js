import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'
import { ServerEnvironment } from '../runtime/ServerEnvironment.js'

export class RawMessagesRecords {
  /**
   * Write message bytes to a private temporary file beside the FYLO root,
   * hand its path to `store`, and remove it afterwards.
   *
   * @template T
   * @param {string} fyloRoot
   * @param {Uint8Array} bytes
   * @param {(sourcePath: string) => Promise<T>} store
   * @returns {Promise<T>}
   */
  static async withStagedSource(fyloRoot, bytes, store) {
    const incomingRoot = resolve(ServerEnvironment.value('FYLO_INCOMING_ROOT') || join(fyloRoot, '.caduceus-incoming'))
    await mkdir(incomingRoot, { recursive: true, mode: 0o700 })
    const temporaryRoot = await mkdtemp(join(incomingRoot, 'raw-message-'))
    const sourcePath = join(temporaryRoot, 'message.eml')
    try {
      await writeFile(sourcePath, bytes, { mode: 0o600 })
      return await store(sourcePath)
    } finally {
      await rm(temporaryRoot, { recursive: true, force: true })
    }
  }
}
