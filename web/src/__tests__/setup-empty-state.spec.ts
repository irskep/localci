import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import SetupEmptyState from '@/components/SetupEmptyState.vue'

describe('SetupEmptyState', () => {
  it('shows the first-run setup commands', () => {
    const wrapper = mount(SetupEmptyState)

    const text = wrapper.text()
    expect(text).not.toContain('No LocalCI runs yet')
    expect(text).toContain('"github:irskep/localci" = "latest"')
    expect(text).toContain('mise install')
    expect(text).toContain('mise-tasks/localci/test')
    expect(text).toContain('chmod +x mise-tasks/localci/test')
    expect(text).toContain('localci start')
    expect(text).toContain('localci install-hooks')
    expect(text).toContain('localci wait')
  })
})
