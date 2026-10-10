// @ts-check

import { Fylo } from '../persistence/Fylo.js'

export class PresenceRules {
  /**
   * Presence tracking — who's online right now, scoped per domain.
   *
   * Persisted in Fylo under {@link Collections.PRESENCE} so heartbeats from any
   * Tachyon worker share the same view (Tachyon spawns a fresh subprocess per
   * request, so a pure in-memory map can't synchronise across endpoints).
   *
   * One document per user — the `email` field is the primary key. Updates are
   * delete-then-put because Fylo's `putData` always inserts under a fresh
   * docId. A user is "online" if `lastSeenAt` is within ONLINE_WINDOW_MS.
   * Records older than PRUNE_AFTER_MS are pruned lazily on snapshot reads.
   *
   * @module presence
   */

  static ONLINE_WINDOW_MS = 60_000

  static PRUNE_AFTER_MS = 10 * 60_000
}
