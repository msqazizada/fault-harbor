export interface ServerStatus {
  version: string
}

async function getJSON(path: string, signal: AbortSignal): Promise<unknown> {
  const response = await fetch(path, { signal, headers: { Accept: 'application/json' } })
  if (!response.ok) throw new Error(`Server returned HTTP ${response.status}.`)
  return response.json()
}

export async function getServerStatus(signal: AbortSignal): Promise<ServerStatus> {
  const boundedSignal = AbortSignal.any([signal, AbortSignal.timeout(5000)])
  const [health, release] = await Promise.all([
    getJSON('/health/ready', boundedSignal),
    getJSON('/api/v1/version', boundedSignal),
  ])

  if (
    typeof health !== 'object' || health === null ||
    !('status' in health) || health.status !== 'ok' ||
    typeof release !== 'object' || release === null ||
    !('version' in release) || typeof release.version !== 'string'
  ) {
    throw new Error('The server returned an unexpected response.')
  }

  return { version: release.version }
}
