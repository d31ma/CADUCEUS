/** @typedef {{ id: string, email: import('./SendRequest.js').SendRequest, sender: string, owner?: string, sendAt: number, createdAt: number, updatedAt?: number, queueId?: string, status?: 'scheduled'|'delivered'|'failed', transport?: 'external', externalAccountId?: string, externalSubmissionId?: string }} ScheduledEntry */
export {}
