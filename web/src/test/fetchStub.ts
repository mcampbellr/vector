import { vi } from 'vitest'

/** A fetch call as the write client issues it. */
export interface RecordedRequest {
  url: string
  method: string
  body: unknown
}

export function jsonResponse(status: number, body: object): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** Stubs global fetch with one queued response per call (the last one repeats)
 *  and records every request. Pair with vi.unstubAllGlobals() in afterEach. */
export function stubFetch(...responses: Array<() => Promise<Response>>) {
  const requests: RecordedRequest[] = []
  let call = 0
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    requests.push({
      url: String(input),
      method: init?.method ?? 'GET',
      body: typeof init?.body === 'string' ? (JSON.parse(init.body) as unknown) : undefined,
    })
    const respond = responses[Math.min(call, responses.length - 1)]
    call += 1
    return respond()
  })
  vi.stubGlobal('fetch', fetchMock)
  return { requests, fetchMock }
}

/** A response the test resolves by hand, to observe the pending state. */
export function deferredResponse(): { promise: () => Promise<Response>; resolve: (response: Response) => void } {
  let resolveResponse: (response: Response) => void = () => {}
  const pending = new Promise<Response>((resolve) => {
    resolveResponse = resolve
  })
  return { promise: () => pending, resolve: (response) => resolveResponse(response) }
}
