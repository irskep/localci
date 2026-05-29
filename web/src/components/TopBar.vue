<script setup lang="ts">
import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import type { BreadcrumbItem } from '@/components/AppBreadcrumbs.vue'
import { useNotificationStore } from '@/stores/notifications'

defineProps<{
  items: BreadcrumbItem[]
}>()

const notifications = useNotificationStore()
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

.top-bar-docs {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-2);
  flex: none;
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
