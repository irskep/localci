<script setup lang="ts">
import { computed, ref } from 'vue'

import { useLocalciStore } from '@/stores/localci'

const props = defineProps<{
  path: string
  apiPath: string
  compact?: boolean
}>()

const store = useLocalciStore()
const copied = ref(false)
const copying = ref(false)
const revealing = ref(false)
const canShowInFinder = computed(() => navigator.platform.toLowerCase().includes('mac'))

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
</script>

<template>
  <div class="artifact-actions">
    <PButton
      v-tooltip.top="copied ? 'Path copied' : 'Copy path'"
      :label="compact ? undefined : copied ? 'Copied' : 'Copy Path'"
      :aria-label="copied ? 'Path copied' : 'Copy path'"
      :icon="copied ? 'pi pi-check' : 'pi pi-copy'"
      size="small"
      severity="secondary"
      outlined
      :loading="copying"
      @click="copyPath"
    />
    <PButton
      v-if="canShowInFinder"
      v-tooltip.top="'Show in Finder'"
      :label="compact ? undefined : 'Show in Finder'"
      aria-label="Show in Finder"
      icon="pi pi-folder-open"
      size="small"
      :loading="revealing"
      @click="showInFinder"
    />
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
