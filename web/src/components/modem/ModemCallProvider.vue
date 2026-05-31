<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import ModemCallBanner from '@/components/modem/ModemCallBanner.vue'
import { provideModemCallSession } from '@/composables/useModemCallSession'
import { useModemPhoneCountry } from '@/composables/useModemPhoneCountry'

const route = useRoute()
const activeModemId = ref('unknown')
const modemId = computed(() => activeModemId.value)
const { phoneCountry } = useModemPhoneCountry(modemId)

watch(
  () => route.params.id,
  (value) => {
    if (typeof value === 'string' && value) {
      activeModemId.value = value
    }
  },
  { immediate: true },
)

const callSession = provideModemCallSession(modemId, phoneCountry)
const remoteAudioRef = ref<HTMLAudioElement | null>(null)

const syncRemoteAudio = async (stream: MediaStream | null) => {
  const audio = remoteAudioRef.value
  if (!audio) return
  if (audio.srcObject !== stream) {
    audio.srcObject = stream
  }
  if (!stream) {
    audio.pause()
    return
  }
  try {
    await audio.play()
  } catch (err) {
    console.warn('[ModemCallProvider] play remote call audio:', err)
  }
}

watch(
  callSession.callAudio.remoteStream,
  (stream) => {
    void syncRemoteAudio(stream)
  },
  { flush: 'post' },
)
</script>

<template>
  <slot />
  <ModemCallBanner :session="callSession" />
  <audio ref="remoteAudioRef" class="sr-only" autoplay playsinline />
</template>
