import { describe, expect, it } from 'bun:test'
import {
  DEFAULT_NOTIFICATION_SOUNDS,
  SOUND_EVENTS,
  SOUND_OPTIONS,
  normalizeNotificationSounds,
  playNotificationSound,
} from '../../client/shared/scripts/notification-sounds.js'
import { NotificationSounds } from '../../server/shared/users/NotificationSounds.js'

describe('notification sounds', () => {
  it('keeps browser and persistence catalogs in lockstep', () => {
    expect(SOUND_EVENTS.map(event => event.id)).toEqual([...NotificationSounds.SOUND_EVENT_IDS])
    expect(SOUND_OPTIONS.map(sound => sound.id)).toEqual([...NotificationSounds.SOUND_IDS])
  })

  it('normalizes unknown or hostile values to local defaults', () => {
    expect(normalizeNotificationSounds({
      'new-mail': 'url(https://sounds.example.test/leak.ogg)',
      'hit-send': 'pop',
    })).toEqual({ ...DEFAULT_NOTIFICATION_SOUNDS, 'hit-send': 'pop' })
    expect(NotificationSounds.validateNotificationSounds({ ...DEFAULT_NOTIFICATION_SOUNDS, surprise: 'bell' })).toMatchObject({ ok: false })
  })

  it('synthesizes fixed cues and treats muted or unavailable audio as a no-op', async () => {
    const played = []
    const scheduled = []
    class FakeAudioContext {
      state = 'running'
      currentTime = 2
      destination = {}
      createOscillator() {
        return {
          type: 'sine',
          frequency: { setValueAtTime: (frequency, time) => scheduled.push(['frequency', frequency, time]) },
          connect() {},
          start: time => scheduled.push(['start', time]),
          stop: time => scheduled.push(['stop', time]),
        }
      }
      createGain() {
        return {
          gain: {
            setValueAtTime: (value, time) => scheduled.push(['gain', value, time]),
            exponentialRampToValueAtTime: (value, time) => scheduled.push(['ramp', value, time]),
          },
          connect() {},
        }
      }
      async resume() {}
    }
    const windowObject = {
      AudioContext: FakeAudioContext,
      dispatchEvent: event => played.push(event.detail),
    }

    expect(await playNotificationSound('new-mail', { soundId: 'bell', windowObject })).toBe(true)
    expect(played).toEqual([{ event: 'new-mail', soundId: 'bell' }])
    expect(scheduled.some(entry => entry[0] === 'frequency')).toBe(true)
    expect(await playNotificationSound('new-mail', { soundId: 'none', windowObject })).toBe(false)
    expect(await playNotificationSound('unknown', { soundId: 'bell', windowObject })).toBe(false)
    expect(await playNotificationSound('send', { soundId: 'pulse', windowObject: {} })).toBe(false)
  })
})
