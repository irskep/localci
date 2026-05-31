export type RouteKind = 'repo' | 'commit' | 'task' | 'attempt' | 'artifact'

export type ParsedRepoRoute = {
  kind: RouteKind
  repoPath: string
  commit?: string
  taskName?: string
  attempt?: number
  artifactPath?: string
  apiPath: string
}

function encodePathSegment(value: string): string {
  return encodeURIComponent(value)
}

export function repoPathURL(repoPath: string): string {
  return `/repo/${repoPath}`
}

export function commitURL(repoPath: string, commit: string): string {
  return `${repoPathURL(repoPath)}/commit/${encodePathSegment(commit)}`
}

export function taskURL(repoPath: string, commit: string, taskName: string): string {
  return `${commitURL(repoPath, commit)}/task/${encodePathSegment(taskName)}`
}

export function attemptURL(
  repoPath: string,
  commit: string,
  taskName: string,
  attempt: number,
): string {
  return `${taskURL(repoPath, commit, taskName)}/attempt/${attempt}`
}

export function artifactURL(
  repoPath: string,
  commit: string,
  taskName: string,
  attempt: number,
  artifactPath: string,
): string {
  const artifactSegments = artifactPath.split('/').map(encodePathSegment).join('/')
  return `${attemptURL(repoPath, commit, taskName, attempt)}/artifact/${artifactSegments}`
}

export function parseRepoRoute(pathname: string): ParsedRepoRoute {
  const trimmed = pathname.replace(/^\/+|\/+$/g, '')
  const segments = trimmed === '' ? [] : trimmed.split('/').map(decodeURIComponent)

  if (segments[0] !== 'repo') {
    throw new Error(`unsupported route: ${pathname}`)
  }

  const commitIndex = segments.indexOf('commit')
  if (commitIndex < 0) {
    const repoPath = segments.slice(1).join('/')
    if (!repoPath) {
      throw new Error(`missing repo path in route: ${pathname}`)
    }
    return {
      kind: 'repo',
      repoPath,
      apiPath: `/api${pathname}`,
    }
  }

  const repoPath = segments.slice(1, commitIndex).join('/')
  const commit = segments[commitIndex + 1]
  if (!commit) {
    throw new Error(`missing commit in route: ${pathname}`)
  }

  if (segments.length === commitIndex + 2) {
    return {
      kind: 'commit',
      repoPath,
      commit,
      apiPath: `/api${pathname}`,
    }
  }

  if (segments[commitIndex + 2] !== 'task') {
    throw new Error(`unsupported commit route: ${pathname}`)
  }

  const taskName = segments[commitIndex + 3]
  if (!taskName) {
    throw new Error(`missing task in route: ${pathname}`)
  }

  if (segments.length === commitIndex + 4) {
    return {
      kind: 'task',
      repoPath,
      commit,
      taskName,
      apiPath: `/api${pathname}`,
    }
  }

  if (segments[commitIndex + 4] !== 'attempt') {
    throw new Error(`unsupported task route: ${pathname}`)
  }

  const attempt = Number.parseInt(segments[commitIndex + 5] ?? '', 10)
  if (!Number.isInteger(attempt) || attempt <= 0) {
    throw new Error(`invalid attempt in route: ${pathname}`)
  }

  if (segments.length === commitIndex + 6) {
    return {
      kind: 'attempt',
      repoPath,
      commit,
      taskName,
      attempt,
      apiPath: `/api${pathname}`,
    }
  }

  if (segments[commitIndex + 6] !== 'artifact') {
    throw new Error(`unsupported attempt route: ${pathname}`)
  }

  const artifactPath = segments.slice(commitIndex + 7).join('/')
  return {
    kind: 'artifact',
    repoPath,
    commit,
    taskName,
    attempt,
    artifactPath,
    apiPath: `/api${pathname}`,
  }
}
