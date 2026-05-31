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
    {{ label(artifact) }}
    <span class="marked-artifact-action">{{ action(artifact) }}</span>
  </a>
</template>

<style scoped>
.marked-artifact-link {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-1);
  padding: 0 var(--app-space-2);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
  background: var(--p-content-hover-background);
  font-size: var(--p-form-field-sm-font-size);
}

.marked-artifact-action {
  color: var(--p-text-muted-color);
}
</style>
