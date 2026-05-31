<script setup lang="ts">
import type { ArtifactView } from '@/lib/api'
import { artifactURL } from '@/lib/routes'

const props = defineProps<{
  artifacts: ArtifactView[]
  repoPath: string
  commit: string
  taskName: string
  attempt: number
}>()

function markedArtifacts(): ArtifactView[] {
  return props.artifacts.filter((artifact) => artifact.marked_name)
}

function label(artifact: ArtifactView): string {
  return artifact.marked_name || artifact.display_name
}

function action(artifact: ArtifactView): string {
  if (artifact.action === 'download') return 'download'
  if (artifact.action === 'reveal') return 'reveal'
  if (artifact.action === 'view') return 'view'
  return 'open'
}

function actionIcon(artifact: ArtifactView): string {
  switch (action(artifact)) {
    case 'download':
      return 'pi pi-download'
    case 'reveal':
      return 'pi pi-folder-open'
    case 'view':
      return 'pi pi-eye'
    default:
      return 'pi pi-external-link'
  }
}

function actionLabel(artifact: ArtifactView): string {
  const value = action(artifact)
  return value.slice(0, 1).toUpperCase() + value.slice(1)
}

function href(artifact: ArtifactView): string {
  if (artifact.action === 'download' && artifact.download_url) return artifact.download_url
  if (artifact.action === 'open' && artifact.raw_url) return artifact.raw_url
  return artifactURL(
    props.repoPath,
    props.commit,
    props.taskName,
    props.attempt || 1,
    artifact.display_name,
  )
}
</script>

<template>
  <a
    v-for="artifact in markedArtifacts()"
    :key="artifact.display_name"
    class="marked-artifact-link"
    :href="href(artifact)"
  >
    <span class="marked-artifact-label">{{ label(artifact) }}</span>
    <i
      class="marked-artifact-action"
      :class="actionIcon(artifact)"
      :title="actionLabel(artifact)"
      :aria-label="actionLabel(artifact)"
    ></i>
  </a>
</template>

<style scoped>
.marked-artifact-link {
  display: inline-flex;
  align-items: center;
  gap: var(--app-inline-icon-gap);
  padding: 0 var(--app-space-2);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
  background: var(--p-content-hover-background);
  font-size: var(--p-form-field-sm-font-size);
}

.marked-artifact-label {
  min-width: 0;
}

.marked-artifact-action {
  color: var(--p-text-muted-color);
  font-size: var(--app-inline-icon-font-size);
}
</style>
