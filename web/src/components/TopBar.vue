<script setup lang="ts">
import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import type { BreadcrumbItem } from '@/components/AppBreadcrumbs.vue'

defineProps<{
  items: BreadcrumbItem[]
}>()
</script>

<template>
  <div class="top-bar">
    <AppBreadcrumbs :items="items" />
    <a
      class="top-bar-docs"
      href="https://steveasleep.com/localci/"
      target="_blank"
      rel="noreferrer"
    >
      <i class="pi pi-book" aria-hidden="true"></i>
      <span>Docs</span>
    </a>
  </div>
</template>

<style scoped>
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--app-space-4);
  margin-bottom: var(--app-space-4);
  min-width: 0;
}

.top-bar-docs {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-2);
  flex: none;
}
</style>
