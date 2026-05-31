<script setup lang="ts">
import { computed } from 'vue'

import { shortCommit } from '@/lib/api'
import { entityKindIcon } from '@/lib/entity-icons'
import { commitURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  commit: string
  label?: string
}>()

const label = computed(() => props.label ?? shortCommit(props.commit))
const icon = entityKindIcon('commit')
</script>

<template>
  <RouterLink :to="commitURL(repoPath, commit)" class="icon-link mono">
    <i :class="icon" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </RouterLink>
</template>
