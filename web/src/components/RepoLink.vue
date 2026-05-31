<script setup lang="ts">
import { computed } from 'vue'

import { entityKindIcon } from '@/lib/entity-icons'
import { repoPathURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  repoLabel?: string
  label?: string
}>()

const label = computed(() => props.label ?? props.repoLabel ?? props.repoPath)
const icon = entityKindIcon('repo')
</script>

<template>
  <RouterLink :to="repoPathURL(repoPath)" class="icon-link">
    <i :class="icon" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </RouterLink>
</template>
