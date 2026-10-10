import { formatPreferenceDate } from '../shared/scripts/mail-preferences.js'
import { formatNumber } from '../shared/scripts/localization.js'
import { resolveSnoozePreset } from '../shared/scripts/snooze-presets.js'

/**
 * Dates, times and sizes in the user's locale, time zone and 12- or 24-hour
 * preference. Go services hold timestamps as ISO text; the browser's Intl
 * and local-time rules, which a WebAssembly service cannot reach, format
 * them here. The view calls these methods, and labels that a Go service
 * writes into text (a quoted reply) come back through acceptDateLabels.
 */
@Service
export default class LocalizationService {
  static _KILOBYTE = 1024
  static _MEGABYTE = 1048576
  static _ONE_DECIMAL = { minimumFractionDigits: 1, maximumFractionDigits: 1 }
  static _DAY_ONLY = { weekday: 'short', month: 'short', day: 'numeric', timeZone: 'UTC' }
  static _DAY_AND_TIME = { weekday: 'short', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }
  static _TIME_ONLY = { hour: '2-digit', minute: '2-digit' }
  static _ALL_DAY = ' · All day'
  static _RANGE = ' – '
  static _DATE_LENGTH = 10

  /** @type {MessageService} */
  message
  /** @type {ConversationService} */
  conversation

  /**
   * @param {string} value
   * @returns {string}
   */
  date(value) {
    return formatPreferenceDate(value)
  }

  /**
   * @param {number} size
   * @returns {string}
   */
  bytes(size) {
    const bytes = this._size(size)
    const [, divisor, unit, options] = [
      [LocalizationService._KILOBYTE, 1, 'B', {}],
      [LocalizationService._MEGABYTE, LocalizationService._KILOBYTE, 'KB', LocalizationService._ONE_DECIMAL],
      [Infinity, LocalizationService._MEGABYTE, 'MB', LocalizationService._ONE_DECIMAL],
    ].find(([limit]) => bytes < Number(limit))
    return `${formatNumber(bytes / Number(divisor), options)} ${unit}`
  }

  /**
   * The invitation's time range. An all-day range shows floating UTC dates
   * and treats the end date as exclusive.
   *
   * @param {CalendarInvite} invite
   * @returns {string}
   */
  inviteWhen(invite) {
    return this._when(this._date(invite?.start), this._date(invite?.end), Boolean(invite?.allDay))
  }

  /**
   * @param {string} preset
   * @returns {string}
   */
  snoozePreset(preset) {
    return this._timestampText(resolveSnoozePreset(preset))
  }

  /**
   * @param {string} preset
   * @returns {string}
   */
  snoozePresetLabel(preset) {
    return this._label(resolveSnoozePreset(preset))
  }

  /**
   * A future datetime-local value as ISO text, or nothing.
   *
   * @param {string} value
   * @returns {string}
   */
  localDateTime(value) {
    return this._timestampText(this._future(new Date(value)))
  }

  /**
   * @param {string} value
   * @returns {string}
   */
  localDateTimeLabel(value) {
    return this._label(this._future(new Date(value)))
  }

  /** @param {DateLabelsRequested} requested */
  @OnEvent
  async labels(requested) {
    const labels = requested.values.map(value => formatPreferenceDate(value))
    await this._owner(requested.owner)?.acceptDateLabels(requested.values, labels)
  }

  /** @param {number} size @returns {number} */
  _size(size) {
    return Math.max(0, Number(size) || 0)
  }

  /** @param {string | undefined} value @returns {Date | null} */
  _date(value) {
    return value ? new Date(value) : null
  }

  /** @param {Date | null} date @returns {string} */
  _timestampText(date) {
    return date?.toISOString() ?? ''
  }

  /** @param {Date | null} date @returns {string} */
  _label(date) {
    return date ? formatPreferenceDate(date) : ''
  }

  /** @param {Date | null} start @param {Date | null} end @param {boolean} allDay @returns {string} */
  _when(start, end, allDay) {
    if (!start) return ''
    return [this._timed, this._allDay][Number(allDay)].call(this, start, end)
  }

  /** @param {Date} start @param {Date | null} end @returns {string} */
  _allDay(start, end) {
    return this._dayRange(start, this._lastDay(start, end))
  }

  /** @param {Date} start @param {Date | null} end @returns {Date} */
  _lastDay(start, end) {
    return end ? new Date(end.getTime() - 1) : start
  }

  /** @param {Date} start @param {Date} last @returns {string} */
  _dayRange(start, last) {
    const day = (/** @type {Date} */ value) => formatPreferenceDate(value, LocalizationService._DAY_ONLY)
    const length = LocalizationService._DATE_LENGTH
    if (last.toISOString().slice(0, length) === start.toISOString().slice(0, length)) return day(start) + LocalizationService._ALL_DAY
    return day(start) + LocalizationService._RANGE + day(last) + LocalizationService._ALL_DAY
  }

  /** @param {Date} start @param {Date | null} end @returns {string} */
  _timed(start, end) {
    if (!end) return formatPreferenceDate(start, LocalizationService._DAY_AND_TIME)
    return this._timedRange(start, end)
  }

  /** @param {Date} start @param {Date} end @returns {string} */
  _timedRange(start, end) {
    const formats = [LocalizationService._DAY_AND_TIME, LocalizationService._TIME_ONLY]
    const endFormat = formats[Number(start.toDateString() === end.toDateString())]
    return formatPreferenceDate(start, LocalizationService._DAY_AND_TIME) + LocalizationService._RANGE + formatPreferenceDate(end, endFormat)
  }

  /** @param {Date} date @returns {Date | null} */
  _future(date) {
    if (Number.isFinite(date.getTime()) && date.getTime() > Date.now()) return date
    return null
  }

  /** @param {string} owner */
  _owner(owner) {
    const services = { message: this.message, conversation: this.conversation }
    return services[/** @type {'message'|'conversation'} */ (owner)]
  }
}
