<script setup lang="ts">
import { useWebsocketStore } from '@/stores/websocket'

const websocket = useWebsocketStore()
</script>

<template>
  <div class="websocket-status">
    <PTag :severity="websocket.severity" :value="websocket.label" />
  </div>
</template>

<style scoped>
.websocket-status {
  display: flex;
  min-width: 0;
}
</style>
