import { expect, test } from 'bun:test'
import { InboundService } from '../../server/features/delivery/services/inbound.js'

/** @returns {InboundService} */
function parser() {
  return Object.create(InboundService.prototype)
}

test('plain inbound content bypasses MIME parsing without losing text', async () => {
  const raw = 'Note: lunch\n\nTomorrow at noon'
  expect(await parser().parseMessage(raw)).toEqual({
    text: raw,
    html: '',
    attachments: [],
    headers: {},
  })
})

test('simple RFC content exposes headers without loading the full MIME parser', async () => {
  const raw = [
    'X-Spam-Flag: YES',
    'Content-Type: text/plain',
    '',
    'Buy now',
  ].join('\r\n')

  expect(await parser().parseMessage(raw)).toEqual({
    text: 'Buy now',
    html: '',
    attachments: [],
    headers: {
      'x-spam-flag': 'YES',
      'content-type': 'text/plain',
    },
  })
})
