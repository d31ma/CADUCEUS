/**
 * Error response shapes. Each status code returns a distinct JSON shape so
 * Tachyon's matchStatusCode can map the response to the right OPTIONS schema.
 * The discriminating fields (`unauthorized`, `forbidden`, `notFound`, ...) keep
 * each status uniquely identifiable in ascending order.
 */
export class Respond {
  /** @param {string} error @returns {{ error: string }} */
  static r400(error) { return { error } }

  /** @param {string} error @returns {{ error: string, unauthorized: true }} */
  static r401(error) { return { error, unauthorized: true } }

  /** @param {string} error @returns {{ error: string, forbidden: true }} */
  static r403(error) { return { error, forbidden: true } }

  /** @param {string} error @returns {{ error: string, notFound: true }} */
  static r404(error) { return { error, notFound: true } }

  /** @param {string} error @param {string[]} blocked @returns {{ error: string, blocked: string[] }} */
  static r422(error, blocked) { return { error, blocked } }

  /** @param {string} error @param {number} retryAfterSeconds @returns {{ error: string, retryAfterSeconds: number, rateLimited: true }} */
  static r429(error, retryAfterSeconds) { return { error, retryAfterSeconds, rateLimited: true } }

  /** @param {string} error @returns {{ error: string, conflict: true }} */
  static r409(error) { return { error, conflict: true } }

  /** @param {string} error @returns {{ error: string, unavailable: true }} */
  static r503(error) { return { error, unavailable: true } }
}
