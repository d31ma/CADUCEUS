export interface MessageEnvelope {
  from: string
  to: string[]
  cc: string[]
  bcc: string[]
  replyTo: string[]
  subject: string
  inReplyTo: string
  references: string[]
  sourceEmailId: string
  openTracking: boolean
  linkTracking: boolean
  remindAt: string
}
