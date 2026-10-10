export class ExternalFolderError extends Error {
  constructor(folder) {
    super(`The connected account does not expose a ${String(folder)} folder`)
    this.name = 'ExternalFolderError'
  }
}
