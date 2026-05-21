import { describe, expect, it } from 'vitest'

import { artifactURL, parseRepoRoute } from '@/lib/routes'

describe('parseRepoRoute', () => {
  it('parses a commit route', () => {
    expect(parseRepoRoute('/repo/cli/localci/commit/abc123')).toMatchObject({
      kind: 'commit',
      repoPath: 'cli/localci',
      commit: 'abc123',
      apiPath: '/api/repo/cli/localci/commit/abc123',
    })
  })

  it('parses an artifact route with encoded task names', () => {
    const url = artifactURL('cli/localci', 'abc123', '//web:localci:test', 2, 'dist/index.html')
    expect(parseRepoRoute(url)).toMatchObject({
      kind: 'artifact',
      repoPath: 'cli/localci',
      commit: 'abc123',
      taskName: '//web:localci:test',
      attempt: 2,
      artifactPath: 'dist/index.html',
    })
  })
})
