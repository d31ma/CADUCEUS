export interface NavigationRequested {
  route: string
  clearRecovery: boolean
  discardDraft: string
  undoId: string
  undoLabel: string
  undoDelay: number
  toast: string
  toastDate: string
}
