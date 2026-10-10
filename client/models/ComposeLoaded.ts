import type { ComposeAttachment } from './ComposeAttachment.ts'

export interface ComposeLoaded {
  reason: string
  from: string
  to: string
  cc: string
  bcc: string
  replyTo: string
  subject: string
  inReplyTo: string
  references: string[]
  scheduledId: string
  sendAt: string
  sourceEmailId: string
  remindAt: string
  openTracking: boolean
  linkTracking: boolean
  draftId: string
  draftClientId: string
  draftVersion: number
  draftStatus: string
  savedSnapshot: boolean
  explicitPrefill: boolean
  recovered: boolean
  meaningful: boolean
  signatureId: string
  attachments: ComposeAttachment[]
}
