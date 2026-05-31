<script setup lang="ts">
import { computed } from 'vue'
import type { MenuItem } from 'primevue/menuitem'

import { entityKindIcon, type EntityKind } from '@/lib/entity-icons'

export type BreadcrumbItem = {
  kind: EntityKind
  label: string
  to?: string
}

const props = defineProps<{
  items: BreadcrumbItem[]
}>()

const model = computed<MenuItem[]>(() =>
  props.items.map((item) => ({
    label: item.label,
    icon: entityKindIcon(item.kind),
    route: item.to,
    to: item.to,
  })),
)
</script>

<template>
  <PBreadcrumb :model="model" aria-label="Breadcrumb" class="breadcrumbs">
    <template #item="{ item, props }">
      <RouterLink v-if="item.route" v-slot="{ href, navigate }" :to="item.route" custom>
        <a :href="href" v-bind="props.action" class="breadcrumb-link" @click="navigate">
          <i v-if="item.icon" :class="item.icon" aria-hidden="true"></i>
          <span v-bind="props.label">{{ item.label }}</span>
        </a>
      </RouterLink>
      <span v-else v-bind="props.action">
        <i v-if="item.icon" :class="item.icon" aria-hidden="true"></i>
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

.breadcrumb-link {
  color: var(--p-primary-color);
}
</style>
