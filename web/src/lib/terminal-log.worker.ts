import { TerminalLogParser } from './terminal-log'
import type { LogWorkerRequest, LogWorkerResponse, PlainLogChunk } from './terminal-log'

const parser = new TerminalLogParser()
const pending: string[] = []
let offset = 0
let scheduled = false

function send(message: LogWorkerResponse): void {
  self.postMessage(message)
}

function processText(): void {
  const started = performance.now()
  const updates = new Map<number, PlainLogChunk>()
  while (pending.length > 0 && performance.now() - started < 8) {
    const text = pending[0]!
    let end = Math.min(offset + 4096, text.length)
    // Prefer line boundaries, but also bound work for very long single lines.
    const newline = text.lastIndexOf('\n', end - 1)
    if (newline >= offset) end = newline + 1
    const chunk = parser.append(text.slice(offset, end))
    updates.set(chunk.index, chunk)
    offset = end
    if (offset === text.length) {
      pending.shift()
      offset = 0
    }
  }
  if (updates.size > 0) send({ type: 'plain', chunks: [...updates.values()] })
  scheduled = pending.length > 0
  if (scheduled) setTimeout(processText, 0)
}

self.onmessage = (event: MessageEvent<LogWorkerRequest>) => {
  const message = event.data
  if (message.type === 'color') {
    send({ type: 'color', request: message.request, chunks: parser.colors(message.indices) })
    return
  }
  if (!message.text) return
  pending.push(message.text)
  if (!scheduled) {
    scheduled = true
    setTimeout(processText, 0)
  }
}
