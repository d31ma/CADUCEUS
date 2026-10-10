import { expect, test } from 'bun:test'
import { EventEmitter } from 'node:events'
import { SubmissionServer } from '../../server/features/delivery/services/smtp-submission-server.js'
import { listenForConnections } from '../../server/platform/runtime/listener.ts'

test('submission SMTP refuses to start without TLS by default', () => {
  expect(() => new SubmissionServer({
    host: '127.0.0.1',
    port: 587,
    secure: false,
    apiUrl: 'http://127.0.0.1:1',
  })).toThrow('SMTP submission requires TLS')
})

test('listener startup surfaces bind failures instead of hanging', async () => {
  const server = new EventEmitter() as EventEmitter & {
    listen(port: number, host: string, callback: () => void): void
  }
  server.listen = () => {
    queueMicrotask(() => {
      server.emit(
        'error',
        Object.assign(new Error('Failed to listen'), { code: 'EADDRINUSE' }),
      )
    })
  }

  await expect(
    listenForConnections(server, 25, '127.0.0.1'),
  ).rejects.toMatchObject({ code: 'EADDRINUSE' })
})
