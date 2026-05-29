<script setup lang="ts">
import { computed, ref } from 'vue'
import type { MenuItem } from 'primevue/menuitem'

import { useLocalciStore } from '@/stores/localci'

const props = defineProps<{
  path: string
  apiPath: string
  rawUrl?: string
  downloadUrl?: string
  mode?: 'buttons' | 'menu'
}>()

const store = useLocalciStore()
const menu = ref()
const copied = ref(false)
const copying = ref(false)
const revealing = ref(false)
const platform = computed(() => navigator.platform.toLowerCase())
const revealLabel = computed(() => {
  if (platform.value.includes('mac')) return 'Show in Finder'
  if (platform.value.includes('win')) return 'Show in Explorer'
  return 'Show in file manager'
})
const menuItems = computed<MenuItem[]>(() => {
  const items: MenuItem[] = []
  if (props.rawUrl) {
    items.push({
      label: 'Open',
      icon: 'pi pi-external-link',
      command: openArtifact,
    })
  }
  if (props.downloadUrl) {
    items.push({
      label: 'Download',
      icon: 'pi pi-download',
      command: downloadArtifact,
    })
  }
  items.push({
    label: copied.value ? 'Path copied' : 'Copy path',
    icon: copied.value ? 'pi pi-check' : 'pi pi-copy',
    command: copyPath,
  })
  items.push({
    label: revealLabel.value,
    icon: 'pi pi-folder-open',
    command: showInFinder,
  })
  return items
})

async function copyPath(): Promise<void> {
  if (copying.value) return
  copying.value = true
  try {
    await navigator.clipboard.writeText(props.path)
    copied.value = true
  } finally {
    copying.value = false
  }
  window.setTimeout(() => {
    copied.value = false
  }, 1200)
}

async function showInFinder(): Promise<void> {
  if (revealing.value) return
  revealing.value = true
  try {
    await store.revealArtifact(props.apiPath)
  } finally {
    revealing.value = false
  }
}

function openArtifact(): void {
  if (!props.rawUrl) return
  window.open(props.rawUrl, '_blank', 'noopener')
}

function downloadArtifact(): void {
  if (!props.downloadUrl) return
  window.location.href = props.downloadUrl
}

function toggleMenu(event: MouseEvent): void {
  menu.value?.toggle(event)
}
</script>

<template>
  <div class="artifact-actions">
    <template v-if="mode === 'menu'">
      <PButton
        v-tooltip.top="'Artifact actions'"
        aria-label="Artifact actions"
        icon="pi pi-ellipsis-v"
        size="small"
        severity="secondary"
        text
        rounded
        :loading="copying || revealing"
        @click="toggleMenu"
      />
      <PMenu ref="menu" :model="menuItems" popup />
    </template>
    <template v-else>
      <PButton
        v-if="rawUrl"
        v-tooltip.top="'Open artifact'"
        label="Open"
        aria-label="Open artifact"
        icon="pi pi-external-link"
        size="small"
        severity="secondary"
        outlined
        @click="openArtifact"
      />
      <PButton
        v-if="downloadUrl"
        v-tooltip.top="'Download artifact'"
        label="Download"
        aria-label="Download artifact"
        icon="pi pi-download"
        size="small"
        severity="secondary"
        outlined
        @click="downloadArtifact"
      />
      <PButton
        v-tooltip.top="copied ? 'Path copied' : 'Copy path'"
        :label="copied ? 'Copied' : 'Copy Path'"
        :aria-label="copied ? 'Path copied' : 'Copy path'"
        :icon="copied ? 'pi pi-check' : 'pi pi-copy'"
        size="small"
        severity="secondary"
        outlined
        :loading="copying"
        @click="copyPath"
      />
      <PButton
        v-tooltip.top="revealLabel"
        :label="revealLabel"
        :aria-label="revealLabel"
        icon="pi pi-folder-open"
        size="small"
        :loading="revealing"
        @click="showInFinder"
      />
    </template>
  </div>
</template>

<style scoped>
.artifact-actions {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-2);
  flex: 0 0 auto;
}
</style>
