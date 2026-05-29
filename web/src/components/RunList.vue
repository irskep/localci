<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

import RunSurface from '@/components/RunSurface.vue'
import type { CommitSummary } from '@/lib/api'

withDefaults(
  defineProps<{
    runs: CommitSummary[]
    repoPath?: string
    repoLabel?: string
    showRepo?: boolean
    newerTo?: RouteLocationRaw
    olderTo?: RouteLocationRaw
    loadingPage?: boolean
  }>(),
  {
    repoPath: undefined,
    repoLabel: undefined,
    showRepo: true,
    newerTo: undefined,
    olderTo: undefined,
    loadingPage: false,
  },
)
</script>

<template>
  <div class="run-list-wrap">
    <TransitionGroup tag="div" name="run-list" class="run-list" appear>
      <RunSurface
        v-for="run in runs"
        :key="`${repoPath ?? run.repo.repo_path}:${run.commit}`"
        :run="run"
        :repo-path="repoPath ?? run.repo.repo_path"
        :repo-label="repoLabel ?? run.repo.repo_label"
        :show-repo="showRepo"
        summary-mode
      />
    </TransitionGroup>

    <div v-if="newerTo || olderTo" class="run-list-pagination" aria-label="Run list pagination">
      <RouterLink v-if="newerTo" :to="newerTo" class="run-list-page-link">
        <PButton
          label="Newer"
          severity="secondary"
          outlined
          icon="pi pi-arrow-left"
          :loading="loadingPage"
        />
      </RouterLink>
      <RouterLink v-if="olderTo" :to="olderTo" class="run-list-page-link">
        <PButton
          label="Older"
          severity="secondary"
          outlined
          icon="pi pi-arrow-right"
          icon-pos="right"
          :loading="loadingPage"
        />
      </RouterLink>
    </div>
  </div>
</template>

<style scoped>
.run-list-wrap {
  display: grid;
  gap: var(--app-space-5);
  min-width: 0;
}

.run-list {
  display: grid;
  align-content: start;
  grid-auto-rows: max-content;
  gap: var(--app-space-5);
  min-width: 0;
}

.run-list-pagination {
  display: flex;
  justify-content: center;
  gap: var(--app-space-3);
  justify-self: center;
}

.run-list-page-link {
  display: inline-flex;
  text-decoration: none;
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
