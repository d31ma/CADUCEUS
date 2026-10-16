import { playNotificationSound } from '../shared/scripts/notification-sounds.js'

/**
 * Plays a notification cue with the Web Audio API for the settings preview.
 * Tachyon has no audio API for WebAssembly services.
 */
@Service
export default class SoundPreviewService {
  /** @type {ApplicationEventPublisher | undefined} Injected before lifecycle methods run. */
  applicationEventPublisher;
  active = true

  /** @param {SoundPreviewRequested} requested */
  @OnEvent
  async play(requested) {
    /** @type {SoundPreviewFinished} */
    const finished = { sound: requested.sound, played: await playNotificationSound(requested.event, { soundId: requested.sound }) }
    ;/** @type {ApplicationEventPublisher} */ (this.applicationEventPublisher).publishEvent(finished, false)
  }
}
