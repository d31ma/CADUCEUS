import { Providers } from '../../scripts/support/Providers.mjs'

/**
 * The one provider container a suite's routes run against, like the
 * per-request graph Tachyon builds. `open` points it at a FYLO root
 * (FYLO_ROOT when omitted); controller calls inject from it.
 */
export class TestContainer {
  /** @type {import('../../scripts/support/Providers.mjs').ProviderContainer | null} */
  static current = null

  /** @param {string} [root] */
  static async open(root) {
    await TestContainer.close()
    TestContainer.current = await Providers.container(root)
    return TestContainer.current
  }

  static async close() {
    const current = TestContainer.current
    TestContainer.current = null
    if (current) await current.close()
  }
}
