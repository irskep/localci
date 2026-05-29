import { computed, onUnmounted, watch } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { toValue } from 'vue'

const suffix = 'localci'

export function useDocumentTitle(title: MaybeRefOrGetter<string>): void {
  const fullTitle = computed(() => {
    const value = toValue(title).trim()
    return value ? `${value} - ${suffix}` : suffix
  })

  const stop = watch(
    fullTitle,
    (value) => {
      document.title = value
    },
    { immediate: true },
  )

  onUnmounted(stop)
}
