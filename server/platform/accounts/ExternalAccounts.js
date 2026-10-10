// @ts-check
import { AuthBridgeError, boundaryCall } from '../auth/Auth.js'
import { Respond } from '../http/Respond.js'

// A 25 MiB MIME source expands once in rawRFC822Base64 and once in the signed
// bridge envelope. Keep the request payload below the Go listener's 64 MiB
// frame ceiling while allowing bounded source/attachment responses.
const ACCOUNT_LIMITS = {
  maxRequestBytes: 47 << 20,
  maxResponseBytes: 64 << 20,
}

/** @param {string} operation @param {Record<string, unknown>} params */
function accountCall(operation, params) {
  return boundaryCall(operation, params, ACCOUNT_LIMITS)
}

/** Thin transport facade for the Go-owned external account boundary. */
export class ExternalAccounts {
  /** @param {string} token */
  static providers(token) {
    return accountCall('accounts.providers', { token })
  }

  /** @param {string} token @param {string} address */
  static discover(token, address) {
    return accountCall('accounts.discover', { token, address })
  }

  /** @param {string} token */
  static list(token) {
    return accountCall('accounts.list', { token })
  }

  /** @param {string} token */
  static contacts(token) {
    return accountCall('accounts.contacts', { token })
  }

  /** @param {string} token */
  static contactBooks(token) {
    return accountCall('accounts.contact_books', { token })
  }

  /** @param {string} token */
  static calendarResources(token) {
    return accountCall('accounts.calendar_resources', { token })
  }

  /** @param {string} token @param {string} id @param {Record<string, unknown>} input */
  static mutateCalendar(token, id, input) {
    return accountCall('accounts.calendar_mutate', { ...input, token, id })
  }

  /** @param {string} token @param {string} id @param {string} mutationId */
  static calendarMutation(token, id, mutationId) {
    return accountCall('accounts.calendar_mutation', { token, id, mutationId })
  }

  /** @param {string} token @param {string} id @param {Record<string, unknown>} input */
  static mutateContact(token, id, input) {
    return accountCall('accounts.contact_mutate', { ...input, token, id })
  }

  /** @param {string} token @param {string} id @param {string} mutationId */
  static contactMutation(token, id, mutationId) {
    return accountCall('accounts.contact_mutation', { token, id, mutationId })
  }

  /** @param {string} token @param {Record<string, unknown>} input */
  static create(token, input) {
    return accountCall('accounts.create', { ...input, token })
  }

  /** @param {string} token @param {Record<string, unknown>} input */
  static validate(token, input) {
    return accountCall('accounts.validate', { ...input, token })
  }

  /** @param {string} token @param {Record<string, unknown>} input */
  static beginMicrosoftOAuth(token, input) {
    return accountCall('accounts.microsoft_oauth_begin', { ...input, token })
  }

  /** @param {string} token @param {string} state @param {string} code */
  static completeMicrosoftOAuth(token, state, code) {
    return accountCall('accounts.microsoft_oauth_complete', { token, state, code })
  }

  /** @param {string} token @param {Record<string, unknown>} input */
  static beginGoogleOAuth(token, input) {
    return accountCall('accounts.google_oauth_begin', { ...input, token })
  }

  /** @param {string} token @param {string} state @param {string} code */
  static completeGoogleOAuth(token, state, code) {
    return accountCall('accounts.google_oauth_complete', { token, state, code })
  }

  /** @param {string} token @param {string} id */
  static get(token, id) {
    return accountCall('accounts.get', { token, id })
  }

  /** @param {string} token @param {string} id @param {number} revision */
  static pause(token, id, revision) {
    return accountCall('accounts.pause', { token, id, revision })
  }

  /** @param {string} token @param {string} id @param {number} revision */
  static resume(token, id, revision) {
    return accountCall('accounts.resume', { token, id, revision })
  }

  /** @param {string} token @param {string} id @param {number} revision @param {Record<string, unknown>} update */
  static updatePresentation(token, id, revision, update) {
    return accountCall('accounts.update_presentation', { ...update, token, id, revision })
  }

  /** @param {string} token @param {string} id @param {number} revision @param {Record<string, unknown>} credentials */
  static reconnect(token, id, revision, credentials) {
    return accountCall('accounts.reconnect', { token, id, revision, credentials })
  }

  /** @param {string} token @param {string} id @param {number} revision */
  static retry(token, id, revision) {
    return accountCall('accounts.retry', { token, id, revision })
  }

  /** @param {string} token @param {string} id */
  static diagnostics(token, id) {
    return accountCall('accounts.diagnostics', { token, id })
  }

  /** @param {string} token @param {string} id */
  static folders(token, id) {
    return accountCall('accounts.folders', { token, id })
  }

  /** @param {string} token @param {string} id @param {string} folder @param {number} limit */
  static messages(token, id, folder, limit) {
    return accountCall('accounts.messages', { token, id, folder, limit })
  }

  /** @param {string} token @param {Record<string, unknown>} input */
  static unifiedMessages(token, input = {}) {
    return accountCall('accounts.unified_messages', { ...input, token })
  }

  /** @param {string} token @param {string} id @param {string} stableId */
  static messageSource(token, id, stableId) {
    return accountCall('accounts.message_source', { token, id, stableId })
  }

  /** Metadata-only exact-owner lookup for bounded bulk actions. */
  static message(token, id, stableId) {
    return accountCall('accounts.message', { token, id, stableId })
  }

  /** @param {string} token @param {string} id @param {Record<string, unknown>} input */
  static mutate(token, id, input) {
    return accountCall('accounts.mutate', { ...input, token, id })
  }

  /** @param {string} token @param {string} id @param {string} mutationId */
  static mutation(token, id, mutationId) {
    return accountCall('accounts.mutation', { token, id, mutationId })
  }

  /** Internal workload operation; the private bridge HMAC is its authority. */
  static claimNotifications(holder, limit = 50) {
    return accountCall('accounts.notification_claim', { holder, limit })
  }

  /** Internal workload operation; never exposed through a bearer route. */
  static completeNotifications(token, ids, delivered, code = '') {
    return accountCall('accounts.notification_complete', { token, ids, delivered, code })
  }

  /** @param {string} token @param {string} id @param {Record<string, unknown>} input */
  static submit(token, id, input) {
    return accountCall('accounts.submit', { ...input, token, id })
  }

  static beginSubmissionBatch(token, id, input) {
    return accountCall('accounts.submit_batch_begin', { ...input, token, id })
  }

  static appendSubmissionBatch(token, id, input) {
    return accountCall('accounts.submit_batch_append', { ...input, token, id })
  }

  static commitSubmissionBatch(token, id, input) {
    return accountCall('accounts.submit_batch_commit', { ...input, token, id })
  }

  static beginSubmissionBatchUpdate(token, id, input) {
    return accountCall('accounts.submit_batch_update_begin', { ...input, token, id })
  }

  static appendSubmissionBatchUpdate(token, id, input) {
    return accountCall('accounts.submit_batch_update_append', { ...input, token, id })
  }

  static commitSubmissionBatchUpdate(token, id, input) {
    return accountCall('accounts.submit_batch_update_commit', { ...input, token, id })
  }

  static abortSubmissionBatchUpdate(token, id, input) {
    return accountCall('accounts.submit_batch_update_abort', { ...input, token, id })
  }

  /** Replace a queued provider submission under its exact journal revision. */
  static updateSubmission(token, id, input) {
    return accountCall('accounts.update_submission', { ...input, token, id })
  }

  /** @param {string} token @param {string} id @param {string} submissionId */
  static cancelSubmission(token, id, submissionId) {
    return accountCall('accounts.cancel_submission', { token, id, submissionId })
  }

  /** @param {string} token @param {string} id @param {string} submissionId */
  static submission(token, id, submissionId) {
    return accountCall('accounts.submission', { token, id, submissionId })
  }

  /** @param {string} token @param {string} id @param {number} revision */
  static remove(token, id, revision) {
    return accountCall('accounts.delete', { token, id, revision })
  }
}

/** Convert only stable boundary failures into Tachyon response shapes. */
export function accountFailure(error) {
  if (!(error instanceof AuthBridgeError)) return Respond.r503('External account service unavailable')
  switch (error.status) {
    case 400: return Respond.r400(error.message)
    case 401: return Respond.r401(error.message)
    case 403: return Respond.r403(error.message)
    case 404: return Respond.r404(error.message)
    case 409: return Respond.r409(error.message)
    case 422: return Respond.r422(error.message, ['credentials'])
    default: return Respond.r503(error.message)
  }
}

/** Bearer tokens always come from Tachyon's authenticated request context. */
export function bearerToken(context) {
  return typeof context?.bearer?.token === 'string' ? context.bearer.token : ''
}
