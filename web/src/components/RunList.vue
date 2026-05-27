<script setup lang="ts">
import RunSurface from '@/components/RunSurface.vue'
import type { CommitSummary } from '@/lib/api'

withDefaults(
  defineProps<{
    runs: CommitSummary[]
    repoPath?: string
    showRepo?: boolean
  }>(),
  {
    repoPath: undefined,
    showRepo: true,
  },
)
</script>

<template>
  <TransitionGroup tag="div" name="run-list" class="run-list" appear>
    <RunSurface
      v-for="run in runs"
      :key="`${repoPath ?? run.repo.repo_path}:${run.commit}`"
      :run="run"
      :repo-path="repoPath ?? run.repo.repo_path"
      :show-repo="showRepo"
    />
  </TransitionGroup>
</template>

<style scoped>
.run-list {
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: var(--app-space-5);
  min-width: 0;
}

.run-list-enter-active,
.run-list-leave-active,
.run-list-move {
  transition:
    opacity 160ms ease,
    transform 160ms ease;
}

.run-list-enter-from,
.run-list-leave-to {
  opacity: 0;
  transform: translateY(calc(var(--app-space-2) * -1));
}

.run-list-leave-active {
  position: absolute;
}
</style>
