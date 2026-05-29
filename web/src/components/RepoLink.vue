<script setup lang="ts">
import { computed } from 'vue'

import { repoPathURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  repoLabel?: string
  label?: string
}>()

const label = computed(() => props.label ?? props.repoLabel ?? props.repoPath)
</script>

<template>
  <RouterLink :to="repoPathURL(repoPath)" class="icon-link">
    <i class="pi pi-folder" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </RouterLink>
</template>
