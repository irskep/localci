<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import type { MenuItem } from 'primevue/menuitem'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import type { BreadcrumbItem } from '@/components/AppBreadcrumbs.vue'
import TopBarLink from '@/components/TopBarLink.vue'
import { repoPathURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'
import { useNotificationStore } from '@/stores/notifications'

defineProps<{
  items: BreadcrumbItem[]
}>()

const router = useRouter()
const store = useLocalciStore()
const notifications = useNotificationStore()
const repoMenu = ref()
const repoMenuItems = computed<MenuItem[]>(() =>
  store.repos.map((repo) => ({
    label: repo.repo_label,
    icon: 'pi pi-folder',
    command: () => router.push(repoPathURL(repo.repo_path)),
  })),
)

onMounted(() => {
  void store.loadRepos()
})

function toggleRepoMenu(event: MouseEvent): void {
  repoMenu.value?.toggle(event)
}
</script>

<template>
  <div class="top-bar">
    <AppBreadcrumbs :items="items" />
    <div class="top-bar-actions">
      <PButton
        class="top-bar-notifications"
        text
        size="small"
        :severity="notifications.severity"
        :icon="notifications.icon"
        :label="notifications.label"
        :aria-label="notifications.label"
        :title="notifications.label"
        :disabled="!notifications.supported"
        @click="notifications.activate"
      />
      <TopBarLink href="https://steveasleep.com/localci/" icon="pi pi-book" label="Docs" />
      <PButton
        v-if="repoMenuItems.length > 0"
        class="top-bar-action top-bar-repos"
        label="Repos"
        icon="pi pi-folder"
        icon-pos="left"
        size="small"
        severity="secondary"
        text
        aria-haspopup="menu"
        aria-label="Open repo menu"
        @click="toggleRepoMenu"
      />
      <PMenu ref="repoMenu" :model="repoMenuItems" popup />
    </div>
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

.top-bar-actions {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-3);
  flex: none;
}

.top-bar-notifications {
  flex: none;
}

.top-bar-repos {
  flex: none;
}

@media (max-width: 640px) {
  .top-bar {
    align-items: flex-start;
    flex-direction: column;
  }

  .top-bar-actions {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
