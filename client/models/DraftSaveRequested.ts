import type { MessageEnvelope } from './MessageEnvelope.ts'

export interface DraftSaveRequested {
  envelope: MessageEnvelope
  draftId: string
  draftClientId: string
  draftVersion: number
}
