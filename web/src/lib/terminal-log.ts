import { AnsiUp } from 'ansi_up'

export type PlainLogChunk = { index: number; version: number; text: string }
export type ColoredLogChunk = { index: number; version: number; html: string }
export type LogWorkerRequest =
  | { type: 'append'; text: string }
  | { type: 'color'; request: number; indices: number[] }
export type LogWorkerResponse =
  | { type: 'plain'; chunks: PlainLogChunk[] }
  | { type: 'color'; request: number; chunks: ColoredLogChunk[] }

// Decode only the escaping performed by AnsiUp, in a single pass. The result is
// inserted with textContent, never as HTML.
function htmlText(html: string): string {
  const entities: Record<string, string> = {
    '&amp;': '&',
    '&lt;': '<',
    '&gt;': '>',
    '&quot;': '"',
    '&#x27;': "'",
  }
  return html
    .replace(/<[^>]*>/g, '')
    .replace(/&(?:amp|lt|gt|quot|#x27);/g, (entity) => entities[entity]!)
}

// Kept in the worker: colored HTML for offscreen chunks never enters the DOM.
export class TerminalLogParser {
  private converter = new AnsiUp()
  private chunks: Array<ColoredLogChunk & { text: string; rawLength: number }> = []
  private newChunk = true

  append(raw: string): PlainLogChunk {
    if (this.newChunk) {
      this.chunks.push({ index: this.chunks.length, version: 0, html: '', text: '', rawLength: 0 })
      this.newChunk = false
    }
    const chunk = this.chunks[this.chunks.length - 1]!
    const html = this.converter.ansi_to_html(raw)
    chunk.html += html
    chunk.text += htmlText(html)
    chunk.rawLength += raw.length
    chunk.version++
    this.newChunk =
      chunk.text.endsWith('\n') && (chunk.rawLength >= 8192 || chunk.text.split('\n').length > 32)
    return { index: chunk.index, version: chunk.version, text: chunk.text }
  }

  colors(indices: number[]): ColoredLogChunk[] {
    return indices.flatMap((index) => {
      const chunk = this.chunks[index]
      return chunk ? [{ index, version: chunk.version, html: chunk.html }] : []
    })
  }
}
