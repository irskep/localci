export type EntityKind = 'home' | 'repo' | 'commit' | 'task' | 'attempt' | 'artifact'

export const entityKindIcons: Record<EntityKind, string | undefined> = {
  home: undefined,
  repo: 'pi pi-folder',
  commit: 'pi pi-circle',
  task: 'pi pi-clock',
  attempt: 'pi pi-refresh',
  artifact: 'pi pi-file',
}

export function entityKindIcon(kind: EntityKind): string | undefined {
  return entityKindIcons[kind]
}
