import { afterEach, expect, it, vi } from 'vitest'
import { getServerStatus } from './api'

afterEach(() => vi.unstubAllGlobals())

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
