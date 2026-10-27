// Limit analytical browser reads without delaying commerce or health requests.
let active = 0
const waiting: (() => void)[] = []
export async function pricingRead<T>(signal: AbortSignal | undefined, read: () => Promise<T>): Promise<T> {
 await new Promise<void>((resolve, reject) => {
  const start = () => { signal?.removeEventListener('abort', abort); if (signal?.aborted) { reject(new DOMException('Aborted', 'AbortError')); next(); return } active++; resolve() }
  const abort = () => { const i = waiting.indexOf(start); if (i >= 0) waiting.splice(i, 1); reject(new DOMException('Aborted', 'AbortError')) }
  if (signal?.aborted) { reject(new DOMException('Aborted', 'AbortError')); return }
  if (active < 3) start()
  else { waiting.push(start); signal?.addEventListener('abort', abort, { once: true }) }
 })
 try { return await read() } finally { active--; next() }
}
function next() { if (active < 3) waiting.shift()?.() }
