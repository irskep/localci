<script setup lang="ts">
import { computed } from 'vue'
import type { MenuItem } from 'primevue/menuitem'

export type BreadcrumbItem = {
  label: string
  to?: string
}

const props = defineProps<{
  items: BreadcrumbItem[]
}>()

const model = computed<MenuItem[]>(() =>
  props.items.map((item) => ({
    label: item.label,
    route: item.to,
    to: item.to,
  })),
)
</script>

<template>
  <PBreadcrumb :model="model" aria-label="Breadcrumb" class="breadcrumbs">
    <template #item="{ item, props }">
      <RouterLink v-if="item.route" v-slot="{ href, navigate }" :to="item.route" custom>
        <a :href="href" v-bind="props.action" @click="navigate">
          <img v-if="item.label === 'Home'" src="/logo.svg" alt="" class="breadcrumb-logo" />
          <span v-bind="props.label">{{ item.label }}</span>
        </a>
      </RouterLink>
      <span v-else v-bind="props.action">
        <img v-if="item.label === 'Home'" src="/logo.svg" alt="" class="breadcrumb-logo" />
        <span v-bind="props.label">{{ item.label }}</span>
      </span>
    </template>
    <template #separator>/</template>
  </PBreadcrumb>
</template>

<style scoped>
.breadcrumbs {
  padding: 0;
  background: transparent;
}

.breadcrumb-logo {
  width: 1em;
  height: 1em;
  flex: none;
}
</style>
