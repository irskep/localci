<script setup lang="ts">
import { onMounted } from 'vue'

import { useNotificationStore } from '@/stores/notifications'

const notifications = useNotificationStore()

onMounted(() => notifications.startRunMonitor())
</script>

<template>
  <div class="app-shell">
    <RouterView />
  </div>
</template>
