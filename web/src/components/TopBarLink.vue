<script setup lang="ts">
defineProps<{
  href: string
  icon: string
  label: string
}>()
</script>

<template>
  <a class="top-bar-link" :href="href">
    <i :class="icon" aria-hidden="true"></i>
    <span>{{ label }}</span>
  </a>
</template>

<style scoped>
.top-bar-link {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-3);
  flex: none;
  font-size: var(--p-form-field-sm-font-size);
  font-weight: 500;
  line-height: 1;
}
</style>
