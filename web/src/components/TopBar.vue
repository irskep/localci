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
  <div class="top-bar">
    <AppBreadcrumbs :items="items" />
    <PMenubar class="top-bar-menu" :model="topBarItems" aria-label="Top navigation" />
  </div>
</template>

<style scoped>
.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--app-space-4);
  margin-bottom: var(--app-page-block-padding);
  min-width: 0;
}

.top-bar-menu {
  flex: none;
}

:global(.top-bar-menu.p-menubar) {
  padding: 0;
  border: 0;
  background: transparent;
}

:global(.top-bar-menu .p-menubar-root-list) {
  gap: var(--app-space-2);
}

:global(.top-bar-menu .p-menubar-item-content) {
  border-radius: var(--p-content-border-radius);
}

:global(.top-bar-menu .p-menubar-item-link) {
  gap: var(--app-space-3);
  padding-block: var(--app-space-2);
}

@media (max-width: 640px) {
  .top-bar {
    align-items: flex-start;
    flex-direction: column;
  }

  .top-bar-menu {
    width: 100%;
  }
}
</style>
