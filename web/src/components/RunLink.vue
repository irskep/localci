<script setup lang="ts">
import { computed } from 'vue'

import { shortCommit } from '@/lib/api'
import { commitURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  commit: string
  label?: string
}>()

const label = computed(() => props.label ?? shortCommit(props.commit))
</script>

<template>
  <RouterLink :to="commitURL(repoPath, commit)" class="icon-link mono">
    <i class="pi pi-play-circle" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </RouterLink>
</template>
