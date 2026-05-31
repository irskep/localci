<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import type { MenuItem } from 'primevue/menuitem'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import type { BreadcrumbItem } from '@/components/AppBreadcrumbs.vue'
import { repoPathURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'
import { useNotificationStore } from '@/stores/notifications'

defineProps<{
  items: BreadcrumbItem[]
}>()

const router = useRouter()
const store = useLocalciStore()
const notifications = useNotificationStore()
const repoMenuItems = computed<MenuItem[]>(() =>
  store.repos.map((repo) => ({
    label: repo.repo_label,
    command: () => router.push(repoPathURL(repo.repo_path)),
  })),
)
const topBarItems = computed<MenuItem[]>(() => {
  const items: MenuItem[] = [
    {
      label: notifications.label,
      icon: notifications.icon,
      disabled: !notifications.supported,
      command: () => notifications.activate(),
    },
    {
      label: 'Docs',
      icon: 'pi pi-book',
      url: 'https://steveasleep.com/localci/',
    },
  ]
  if (repoMenuItems.value.length > 0) {
    items.push({
      label: 'Repos',
      items: repoMenuItems.value,
    })
  }
  return items
})

onMounted(() => {
  void store.loadRepos()
})
</script>

<template>
  <PMenubar class="top-bar" :model="topBarItems" aria-label="Top navigation">
    <template #start>
      <img src="/logo.svg" alt="LocalCI" class="top-bar-logo" />
      <AppBreadcrumbs :items="items" />
      <slot name="left-actions"></slot>
    </template>
  </PMenubar>
</template>

<style scoped>
:global(.top-bar.p-menubar) {
  gap: var(--app-space-4);
  margin-bottom: var(--app-page-block-padding);
  padding: 0;
  border: 0;
  background: transparent;
  min-width: 0;
}

:global(.top-bar .p-menubar-start) {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-5);
  min-width: 0;
  margin-right: auto;
}

.top-bar-logo {
  display: block;
  flex: none;
  width: auto;
  height: 40px;
}

:global(.top-bar .p-menubar-root-list) {
  gap: var(--app-space-2);
}

:global(.top-bar .p-menubar-item-content) {
  border-radius: var(--p-content-border-radius);
}

:global(.top-bar .p-menubar-item-link) {
  gap: var(--app-space-3);
  padding-block: var(--app-space-2);
}

@media (max-width: 640px) {
  :global(.top-bar .p-menubar-start) {
    width: 100%;
  }
}
</style>
