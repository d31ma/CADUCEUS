import { RouteRequest } from '../shared/http/RouteRequest.js'
import { Fylo } from '../shared/persistence/Fylo.js'

@RequestRoute("/test/reset")
@Public
export class TestResetController {
  /** @type {FyloHelper} */
  @Needs
  fyloHelper

  /**
 * DELETE /test/reset
 *
 * Wipe every Fylo collection. Only enabled when CADUCEUS_ENABLE_TEST_ROUTES
 * and NODE_ENV=test (enforced by the `testOnly` route check) — production builds
 * reject this route at the entrypoint.
 *
 * @returns {Promise<Record<string, unknown>>}
 */
  @DeleteRoute
  async delete(request) {
    return RouteRequest.handle(request, { testOnly: true }, async () => {
    const fylo = await this.fyloHelper.database()
    for (const collection of [...Object.values(Fylo.Collections), ...Object.values(Fylo.Buckets)]) {
      /** @type {Record<string, unknown>} */
      const docs = {}
      for await (const doc of fylo.findDocs(collection, { $ops: [] }).collect()) {
        Object.assign(docs, doc)
      }
      for (const docId of Object.keys(docs)) {
        await fylo.delDoc(collection, docId)
      }
    }
    return { reset: true }
    })
  }
}
