<script setup lang="ts">
withDefaults(
  defineProps<{
    subject: string
    size?: 'compact' | 'large'
  }>(),
  {
    size: 'compact',
  },
)
</script>

<template>
  <div class="commit-subject" :class="`commit-subject-${size}`">
    {{ subject }}
  </div>
</template>

<style scoped>
.commit-subject {
  min-width: 0;
  max-width: 100%;
  padding: var(--app-space-1) var(--app-space-2);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-border-radius-sm);
  background: var(--p-content-hover-background);
  color: var(--p-text-muted-color);
  font-family: var(--app-mono-font-family);
}

.commit-subject-compact {
  width: fit-content;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: var(--p-form-field-sm-font-size);
}

.commit-subject-large {
  width: fit-content;
  padding: var(--app-space-2) var(--app-space-3);
  overflow-wrap: anywhere;
  font-size: var(--p-form-field-lg-font-size);
}
</style>
