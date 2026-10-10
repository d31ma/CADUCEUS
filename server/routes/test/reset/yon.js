import { Guards } from '../../../middleware/guards.js'
import { Buckets, Collections } from '../../../platform/persistence/fylo.js'
import { getApp } from '../../../app.js'

export class Handler {
  /**
 * DELETE /test/reset
 *
 * Wipe every Fylo collection. Only enabled when CADUCEUS_ENABLE_TEST_ROUTES
 * and NODE_ENV=test (enforced by the `@Guards.testOnly` guard) — production builds
 * reject this route at the entrypoint.
 *
 * @returns {Promise<Record<string, unknown>>}
 */
  @Guards.testOnly
  static async DELETE() {
  const app = await getApp()
  for (const collection of [...Object.values(Collections), ...Object.values(Buckets)]) {
    /** @type {Record<string, unknown>} */
    const docs = {}
    for await (const doc of app.fylo.findDocs(collection, { $ops: [] }).collect()) {
      Object.assign(docs, doc)
    }
    for (const docId of Object.keys(docs)) {
      await app.fylo.delDoc(collection, docId)
    }
  }
  return { reset: true }
}
}
