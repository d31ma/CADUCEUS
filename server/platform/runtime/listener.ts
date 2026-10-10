export interface ListenableServer {
  once(event: 'error', listener: (error: Error) => void): unknown
  off(event: 'error', listener: (error: Error) => void): unknown
  listen(port: number, host: string, callback: () => void): unknown
}

/**
 * Resolve only after a server is listening and reject bind failures.
 *
 * Several Node-style server wrappers forward listener errors through their
 * own EventEmitter instead of a listen callback. Keeping this boundary shared
 * prevents process startup from hanging forever on errors such as EADDRINUSE.
 */
export function listenForConnections(
  server: ListenableServer,
  port: number,
  host: string,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const onError = (error: Error) => reject(error)
    server.once('error', onError)
    try {
      server.listen(port, host, () => {
        server.off('error', onError)
        resolve()
      })
    } catch (cause) {
      server.off('error', onError)
      reject(cause)
    }
  })
}
