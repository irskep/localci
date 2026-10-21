<script setup lang="ts">
import { onBeforeUnmount, onMounted, useTemplateRef, watch } from 'vue'
import type { LogWorkerRequest, LogWorkerResponse } from '@/lib/terminal-log'

const props = withDefaults(defineProps<{ text: string; empty?: string }>(), { empty: '' })
const output = useTemplateRef<HTMLPreElement>('output')
const chunks: Array<{
  element: HTMLSpanElement
  plainText: Text
  overlay?: HTMLSpanElement
  version: number
}> = []
const visible = new Set<number>()
const colored = new Set<number>()
let worker: Worker | null = null
let observer: IntersectionObserver | null = null
let previousText = ''
let request = 0
let timer: ReturnType<typeof setTimeout> | undefined
let frame = 0
let scrolling = false

function hasSelection(): boolean {
  const selection = window.getSelection()
  return (
    !!selection &&
    !selection.isCollapsed &&
    (!!output.value?.contains(selection.anchorNode) ||
      !!output.value?.contains(selection.focusNode))
  )
}

function plain(index: number): void {
  const chunk = chunks[index]
  if (chunk && colored.delete(index)) {
    chunk.overlay?.remove()
    chunk.overlay = undefined
  }
}

function send(message: LogWorkerRequest): void {
  worker?.postMessage(message)
}

function scheduleColors(): void {
  if (timer !== undefined) return
  timer = setTimeout(() => {
    timer = undefined
    scrolling = false
    if (hasSelection()) return
    for (const index of colored) {
      if (!visible.has(index)) plain(index)
    }
    output.value?.removeAttribute('data-scrolling')
    send({ type: 'color', request: ++request, indices: [...visible] })
  }, 120)
}

function onScroll(): void {
  scrolling = true
  output.value?.setAttribute('data-scrolling', '')
  request++ // Discard colors requested for a previous viewport.
  clearTimeout(timer)
  timer = undefined
  if (!frame) {
    frame = requestAnimationFrame(() => {
      frame = 0
      scheduleColors()
    })
  }
}

function onSelectionChange(): void {
  output.value?.toggleAttribute('data-selecting', hasSelection())
  if (!hasSelection()) scheduleColors()
}

function reset(): void {
  worker?.terminate()
  observer?.disconnect()
  clearTimeout(timer)
  timer = undefined
  cancelAnimationFrame(frame)
  frame = 0
  scrolling = false
  request++
  chunks.length = 0
  visible.clear()
  colored.clear()
  previousText = ''
  const element = output.value
  if (!element) return
  element.removeAttribute('data-scrolling')
  element.removeAttribute('data-selecting')
  element.textContent = props.text ? '' : props.empty
  observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        const index = Number((entry.target as HTMLElement).dataset.logChunk)
        if (entry.isIntersecting) visible.add(index)
        else {
          visible.delete(index)
        }
      }
      if (!scrolling) scheduleColors()
    },
    { root: element, rootMargin: '400px 0px' },
  )

  const currentWorker = new Worker(new URL('../lib/terminal-log.worker.ts', import.meta.url), {
    type: 'module',
  })
  worker = currentWorker
  currentWorker.onmessage = (event: MessageEvent<LogWorkerResponse>) => {
    if (worker !== currentWorker) return
    const message = event.data
    if (message.type === 'color') {
      if (message.request !== request || scrolling || hasSelection()) return
      for (const update of message.chunks) {
        const chunk = chunks[update.index]
        if (!chunk || chunk.version !== update.version || !visible.has(update.index)) continue
        // Only worker-generated AnsiUp HTML is inserted. HTML escaping stays on.
        if (!chunk.overlay) {
          chunk.overlay = document.createElement('span')
          chunk.overlay.dataset.logColor = ''
          chunk.overlay.setAttribute('aria-hidden', 'true')
          chunk.overlay.setAttribute('inert', '')
          chunk.element.append(chunk.overlay)
        }
        chunk.overlay.innerHTML = update.html
        colored.add(update.index)
      }
      return
    }
    for (const update of message.chunks) {
      let chunk = chunks[update.index]
      if (!chunk) {
        if (!chunks.length) element.textContent = ''
        const span = document.createElement('span')
        span.dataset.logChunk = String(update.index)
        const plainText = document.createTextNode('')
        span.append(plainText)
        element.append(span)
        chunk = { element: span, plainText, version: 0 }
        chunks[update.index] = chunk
        observer?.observe(span)
      }
      chunk.version = update.version
      chunk.plainText.data = update.text
      plain(update.index)
    }
    if (!scrolling) scheduleColors()
  }
  currentWorker.onerror = () => {
    if (worker !== currentWorker) return
    // Keep the log accessible even if the worker cannot be loaded.
    element.textContent = props.text || props.empty
    currentWorker.terminate()
    worker = null
  }
}

function render(): void {
  if (!output.value) return
  if (!worker || !props.text.startsWith(previousText) || !props.text) reset()
  const addition = props.text.slice(previousText.length)
  previousText = props.text
  if (addition) send({ type: 'append', text: addition })
}

onMounted(() => {
  render()
  document.addEventListener('selectionchange', onSelectionChange)
})
watch(() => [props.text, props.empty], render, { flush: 'post' })
onBeforeUnmount(() => {
  worker?.terminate()
  worker = null
  observer?.disconnect()
  clearTimeout(timer)
  cancelAnimationFrame(frame)
  document.removeEventListener('selectionchange', onSelectionChange)
})
</script>

<template>
  <pre ref="output" @scroll.passive="onScroll"></pre>
</template>

<style scoped>
/* Chunks end at line boundaries. Isolating their layout prevents a color
   update from reshaping the entire log as one enormous inline paragraph. */
pre :deep([data-log-chunk]) {
  display: block;
  position: relative;
  contain: layout style;
}

/* The plain text is permanent. Colors never participate in its layout or
   selection and are hidden with a single style change during scrolling. */
pre :deep([data-log-color]) {
  position: absolute;
  inset: 0;
  background: var(--p-content-background);
  pointer-events: none;
  user-select: none;
}

pre[data-scrolling] :deep([data-log-color]),
pre[data-selecting] :deep([data-log-color]) {
  display: none;
}

/* Match the plain layer's font metrics exactly, including wrapped lines. */
pre :deep([data-log-chunk] *) {
  font: inherit !important;
  letter-spacing: inherit !important;
}
</style>
