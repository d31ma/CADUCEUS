/**
 * Publishes the facts about this browser that WebAssembly services cannot
 * read: whether keys show as on Apple devices, the time zone offset dates
 * are written in, and the page origin.
 */
@Service
export default class DevicePlatformService {
  /** @type {ApplicationEventPublisher} */
  applicationEventPublisher;

  @OnStart
  start() {
    /** @type {DevicePlatform} */
    const platform = {
      apple: /Mac|iPhone|iPad/i.test(navigator.platform),
      timeZoneOffset: -new Date().getTimezoneOffset(),
      origin: location.origin,
    }
    this.applicationEventPublisher.publishEvent(platform, true)
  }
}
