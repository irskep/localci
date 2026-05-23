<script setup lang="ts">
import { computed } from 'vue'

import { repoPathURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  label?: string
}>()

const label = computed(() => props.label ?? props.repoPath)
</script>

<template>
  <RouterLink :to="repoPathURL(repoPath)" class="icon-link">
    <i class="pi pi-folder" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </RouterLink>
</template>
