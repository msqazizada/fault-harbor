import { afterEach, expect, it, vi } from 'vitest'
import { getServerStatus } from './api'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

it('reads readiness and version from the backend', async () => {
  vi.stubGlobal('fetch', vi.fn()
    .mockResolvedValueOnce(Response.json({ status: 'ok' }))
    .mockResolvedValueOnce(Response.json({ version: 'test' })))
  await expect(getServerStatus(new AbortController().signal)).resolves.toEqual({ version: 'test' })
})

it('rejects failed requests rather than showing a healthy server', async () => {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 503 })))
  await expect(getServerStatus(new AbortController().signal)).rejects.toThrow('HTTP 503')
})

it('rejects malformed or incompatible API payloads', async () => {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(Response.json({ unrelated: true }))))
  await expect(getServerStatus(new AbortController().signal)).rejects.toThrow('unexpected response')
})

it('does not accept an unavailable readiness state', async () => {
  vi.stubGlobal('fetch', vi.fn()
    .mockResolvedValueOnce(Response.json({ status: 'unavailable' }))
    .mockResolvedValueOnce(Response.json({ version: 'test' })))
  await expect(getServerStatus(new AbortController().signal)).rejects.toThrow('unexpected response')
})

it('propagates network failures', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Network unavailable')))
  await expect(getServerStatus(new AbortController().signal)).rejects.toThrow('Network unavailable')
})

it('rejects a non-JSON response', async () => {
  vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response('<html>Proxy error</html>'))))
  await expect(getServerStatus(new AbortController().signal)).rejects.toThrow(SyntaxError)
})

it.each(['caller', 'deadline'] as const)('cancels both requests on %s cancellation', async (origin) => {
  const caller = new AbortController()
  const deadline = new AbortController()
  const timeout = vi.spyOn(AbortSignal, 'timeout').mockReturnValue(deadline.signal)
  const signals: AbortSignal[] = []
  vi.stubGlobal('fetch', vi.fn((_path: string, { signal }: { signal: AbortSignal }) => {
    signals.push(signal)
    return new Promise<Response>((_resolve, reject) => {
      signal.addEventListener('abort', () => reject(new DOMException('Cancelled', 'AbortError')), { once: true })
    })
  }))
  const result = getServerStatus(caller.signal)
  const rejected = expect(result).rejects.toThrow('Cancelled')
  const cancellation = origin === 'caller' ? caller : deadline
  cancellation.abort()
  await rejected
  expect(signals).toHaveLength(2)
  expect(signals.every((signal) => signal.aborted)).toBe(true)
  expect(timeout).toHaveBeenCalledWith(5000)
})
