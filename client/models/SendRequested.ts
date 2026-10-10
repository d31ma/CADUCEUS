import type { MessageEnvelope } from './MessageEnvelope.ts'

export interface SendRequested {
  mode: string
  envelope: MessageEnvelope
  draftId: string
  draftClientId: string
  scheduledId: string
  sendAt: string
  delayMs: number
  idempotencyKey: string
}
