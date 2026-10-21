import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import TerminalLog from '@/components/TerminalLog.vue'
import { TerminalLogParser } from '@/lib/terminal-log'
import type { LogWorkerRequest, LogWorkerResponse } from '@/lib/terminal-log'

class FakeWorker {
  static instances: FakeWorker[] = []
  onmessage: ((event: MessageEvent<LogWorkerResponse>) => void) | null = null
  onerror: (() => void) | null = null
  messages: LogWorkerRequest[] = []
  terminated = false
  constructor() {
    FakeWorker.instances.push(this)
  }
  postMessage(message: LogWorkerRequest): void {
    this.messages.push(message)
  }
  terminate(): void {
    this.terminated = true
  }
  reply(message: LogWorkerResponse): void {
    this.onmessage?.({ data: message } as MessageEvent<LogWorkerResponse>)
  }
}

class FakeObserver {
  static instances: FakeObserver[] = []
  callback: IntersectionObserverCallback
  observe = vi.fn<(target: Element) => void>()
  disconnect = vi.fn<() => void>()
  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    FakeObserver.instances.push(this)
  }
  intersect(element: Element, isIntersecting = true): void {
    this.callback(
      [{ target: element, isIntersecting } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver,
    )
  }
}

describe('terminal parser (runs in a worker)', () => {
  it('preserves text and carries color state across chunks and split escape sequences', () => {
    const parser = new TerminalLogParser()
    const first = parser.append('\x1b[32m' + 'first\n'.repeat(33))
    const second = parser.append('second\x1b[')
    const third = parser.append('0m plain')
    expect(first.text).toBe('first\n'.repeat(33))
    expect(second.index).toBe(1)
    expect(third.text).toBe('second plain')
    expect(parser.colors([1])[0]?.html).toContain('color:rgb(0,187,0)')
    expect(parser.colors([1])[0]?.html).toMatch(/<\/span> plain$/)
  })

  it('escapes markup and decodes plain text once without interpreting it', () => {
    const parser = new TerminalLogParser()
    const raw = '<img src=x onerror=alert(1)>&lt;"\'🦊'
    expect(parser.append('\x1b[31m' + raw).text).toBe(raw)
    const html = parser.colors([0])[0]!.html
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;img')
    const container = document.createElement('div')
    container.innerHTML = html
    expect(container.textContent).toBe(raw)
    expect(container.querySelector('img')).toBeNull()
  })

  it('supports RGB, resets, and safe terminal links', () => {
    const parser = new TerminalLogParser()
    const plain = parser.append(
      '\x1b[38;2;12;34;56mcolor\x1b[0m plain ' +
        '\x1b]8;;https://example.com/?a=1&b=2\x07link\x1b]8;;\x07',
    )
    expect(plain.text).toBe('color plain link')
    expect(parser.colors([0])[0]?.html).toContain('color:rgb(12,34,56)')
    const unsafe = new TerminalLogParser()
    unsafe.append('\x1b]8;;javascript:alert(1)\x07click\x1b]8;;\x07')
    expect(unsafe.colors([0])[0]?.html).not.toContain('<a')
  })

  it('returns colors only for requested chunks', () => {
    const parser = new TerminalLogParser()
    for (let index = 0; index < 100; index++) parser.append('\x1b[31mline\n'.repeat(33))
    expect(parser.colors([50])).toHaveLength(1)
    expect(parser.colors([50])[0]?.index).toBe(50)
    expect(parser.colors([1000])).toEqual([])
  })
})

describe('progressive terminal rendering', () => {
  const wrappers: Array<ReturnType<typeof mount>> = []
  beforeEach(() => {
    vi.useFakeTimers()
    FakeWorker.instances = []
    FakeObserver.instances = []
    vi.stubGlobal('Worker', FakeWorker)
    vi.stubGlobal('IntersectionObserver', FakeObserver)
    vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) =>
      setTimeout(() => callback(0), 16),
    )
    vi.stubGlobal('cancelAnimationFrame', clearTimeout)
  })
  afterEach(() => {
    for (const wrapper of wrappers) wrapper.unmount()
    wrappers.length = 0
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  function setup(text = 'raw log') {
    const wrapper = mount(TerminalLog, { props: { text, empty: 'No output' } })
    wrappers.push(wrapper)
    return { wrapper, worker: FakeWorker.instances[0]!, observer: FakeObserver.instances[0]! }
  }

  it('shows plain text before requesting colors, and colors only visible chunks', () => {
    const { wrapper, worker, observer } = setup()
    worker.reply({
      type: 'plain',
      chunks: [
        { index: 0, version: 1, text: 'visible\n' },
        { index: 1, version: 1, text: 'offscreen' },
      ],
    })
    expect(wrapper.text()).toBe('visible\noffscreen')
    expect(wrapper.find('[style]').exists()).toBe(false)
    observer.intersect(wrapper.find('[data-log-chunk="0"]').element)
    vi.advanceTimersByTime(120)
    const request = worker.messages[worker.messages.length - 1]!
    expect(request).toMatchObject({ type: 'color', indices: [0] })
    if (request.type !== 'color') throw new Error('missing color request')
    worker.reply({
      type: 'color',
      request: request.request,
      chunks: [{ index: 0, version: 1, html: '<span style="color:red">visible\n</span>' }],
    })
    expect(wrapper.find('[style]').exists()).toBe(true)
    expect(wrapper.find('[data-log-chunk="0"]').element.firstChild?.textContent).toBe('visible\n')
    expect(wrapper.find('[data-log-color]').attributes()).toMatchObject({
      'aria-hidden': 'true',
      inert: '',
    })
    observer.intersect(wrapper.find('[data-log-chunk="0"]').element, false)
    vi.advanceTimersByTime(120)
    expect(wrapper.find('[style]').exists()).toBe(false)
    expect(wrapper.text()).toBe('visible\noffscreen')
  })

  it('falls back to plain text during scrolling and rejects stale color replies', async () => {
    const { wrapper, worker, observer } = setup()
    worker.reply({ type: 'plain', chunks: [{ index: 0, version: 1, text: 'line' }] })
    observer.intersect(wrapper.find('[data-log-chunk]').element)
    vi.advanceTimersByTime(120)
    const request = worker.messages[worker.messages.length - 1]!
    if (request.type !== 'color') throw new Error('missing color request')
    const reply: LogWorkerResponse = {
      type: 'color',
      request: request.request,
      chunks: [{ index: 0, version: 1, html: '<span style="color:red">line</span>' }],
    }
    worker.reply(reply)
    expect(wrapper.find('[style]').exists()).toBe(true)
    const plainNode = wrapper.find('[data-log-chunk]').element.firstChild
    await wrapper.trigger('scroll')
    vi.advanceTimersByTime(16)
    worker.reply(reply)
    expect(wrapper.attributes()).toHaveProperty('data-scrolling')
    expect(wrapper.find('[data-log-chunk]').element.firstChild).toBe(plainNode)
    expect(plainNode?.textContent).toBe('line')
    const count = worker.messages.length
    vi.advanceTimersByTime(119)
    expect(worker.messages).toHaveLength(count)
    vi.advanceTimersByTime(1)
    expect(worker.messages).toHaveLength(count + 1)
  })

  it('sends only appended text and ignores colors for superseded chunk versions', async () => {
    const { wrapper, worker, observer } = setup('first')
    worker.reply({ type: 'plain', chunks: [{ index: 0, version: 1, text: 'first' }] })
    observer.intersect(wrapper.find('[data-log-chunk]').element)
    vi.advanceTimersByTime(120)
    const request = worker.messages[worker.messages.length - 1]!
    if (request.type !== 'color') throw new Error('missing color request')
    await wrapper.setProps({ text: 'first second' })
    expect(worker.messages[worker.messages.length - 1]).toEqual({ type: 'append', text: ' second' })
    worker.reply({ type: 'plain', chunks: [{ index: 0, version: 2, text: 'first second' }] })
    worker.reply({
      type: 'color',
      request: request.request,
      chunks: [{ index: 0, version: 1, html: '<span>first</span>' }],
    })
    expect(wrapper.text()).toBe('first second')
    expect(wrapper.find('[data-log-chunk] span').exists()).toBe(false)
  })

  it('terminates old workers on replacement/unmount and ignores their replies', async () => {
    const { wrapper, worker } = setup('old')
    await wrapper.setProps({ text: 'new' })
    expect(worker.terminated).toBe(true)
    const replacement = FakeWorker.instances[1]!
    worker.reply({ type: 'plain', chunks: [{ index: 0, version: 1, text: 'stale' }] })
    expect(wrapper.text()).toBe('')
    replacement.reply({ type: 'plain', chunks: [{ index: 0, version: 1, text: 'new' }] })
    expect(wrapper.text()).toBe('new')
    await wrapper.setProps({ text: '' })
    expect(wrapper.text()).toBe('No output')
    wrapper.unmount()
    expect(FakeWorker.instances[FakeWorker.instances.length - 1]?.terminated).toBe(true)
  })

  it('leaves an active text selection intact when colors arrive', () => {
    const { wrapper, worker, observer } = setup()
    worker.reply({ type: 'plain', chunks: [{ index: 0, version: 1, text: 'selected' }] })
    const chunk = wrapper.find('[data-log-chunk]').element
    observer.intersect(chunk)
    vi.advanceTimersByTime(120)
    const request = worker.messages[worker.messages.length - 1]!
    if (request.type !== 'color') throw new Error('missing color request')
    vi.spyOn(window, 'getSelection').mockReturnValue({
      isCollapsed: false,
      anchorNode: chunk.firstChild,
      focusNode: chunk.firstChild,
    } as Selection)
    worker.reply({
      type: 'color',
      request: request.request,
      chunks: [{ index: 0, version: 1, html: '<span>selected</span>' }],
    })
    expect(wrapper.find('[data-log-chunk] span').exists()).toBe(false)
    vi.restoreAllMocks()
  })
})
