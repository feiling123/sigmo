<script setup lang="ts">
import type { AlertDialogContentEmits, AlertDialogContentProps } from 'reka-ui'
import type { HTMLAttributes } from 'vue'
import { reactiveOmit } from '@vueuse/core'
import { useForwardPropsEmits } from 'reka-ui'

import { AlertDialogContent } from '@/components/ui/alert-dialog'

defineOptions({
  inheritAttrs: false,
})

const props = defineProps<AlertDialogContentProps & {
  class?: HTMLAttributes['class']
}>()
const emits = defineEmits<AlertDialogContentEmits>()

const delegatedProps = reactiveOmit(props, 'class')
const forwarded = useForwardPropsEmits(delegatedProps, emits)

const preventDialogDismiss = (event: Event) => {
  event.preventDefault()
}
</script>

<template>
  <AlertDialogContent
    v-bind="{ ...$attrs, ...forwarded }"
    :class="props.class"
    @interact-outside="preventDialogDismiss"
    @escape-key-down="preventDialogDismiss"
  >
    <slot />
  </AlertDialogContent>
</template>
