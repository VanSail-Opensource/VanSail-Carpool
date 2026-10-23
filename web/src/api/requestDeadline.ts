/** Bound finite API work, including body reads, without newer AbortSignal APIs. */
export async function withRequestDeadline<T>(milliseconds: number, parent: AbortSignal | null | undefined, load: (signal: AbortSignal) => Promise<T>): Promise<T> {
  const controller = new globalThis.AbortController()
  let timer: ReturnType<typeof setTimeout> | undefined
  let onAbort: (() => void) | undefined
  const interrupted = new Promise<never>((_, reject) => {
    const stop = (reason: unknown) => { controller.abort(); reject(reason) }
    onAbort = () => stop(parent?.reason ?? new DOMException('REQUEST_ABORTED', 'AbortError'))
    if (parent?.aborted) { onAbort(); return }
    parent?.addEventListener('abort', onAbort, { once: true })
    timer = setTimeout(() => stop(new DOMException('REQUEST_TIMEOUT', 'TimeoutError')), milliseconds)
  })
  try {
    if (controller.signal.aborted) return await interrupted
    return await Promise.race([load(controller.signal), interrupted])
  } finally {
    if (timer !== undefined) clearTimeout(timer)
    if (onAbort) parent?.removeEventListener('abort', onAbort)
  }
}
