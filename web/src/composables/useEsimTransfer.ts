import { computed, onBeforeUnmount, ref, type Ref } from 'vue'

import { useEsimApi } from '@/apis/esim'
import {
  TRANSFER_STAGE,
  TRANSFER_STATE,
  transferStageProgress,
  type EsimTransferState,
} from '@/constants/esimTransfer'
import {
  useEsimTransferSession,
  type TransferUserInput,
} from '@/composables/useEsimTransferSession'
import type {
  EsimDownloadPreview,
  EsimTransferProfile,
  EsimTransferSource,
} from '@/types/esim'

type Options = {
  onCompleted?: () => void
}

export const useEsimTransfer = (modemId: Readonly<Ref<string>>, options?: Options) => {
  const esimApi = useEsimApi()
  const state = ref<EsimTransferState>(TRANSFER_STATE.idle)
  const stage = ref('')
  const progress = ref(0)
  const errorMessage = ref('')
  const ccidError = ref('')
  const sources = ref<EsimTransferSource[]>([])
  const profiles = ref<EsimTransferProfile[]>([])
  const selectedSource = ref<EsimTransferSource | null>(null)
  const selectedProfile = ref<EsimTransferProfile | null>(null)
  const sourceImei = ref('')
  const userInput = ref<TransferUserInput | null>(null)
  const userInputResponse = ref('')
  const sourceDeletionICCID = ref('')
  const previewProfile = ref<EsimDownloadPreview | null>(null)

  const hasSources = computed(() => sources.value.length > 0)
  const downloadedName = computed(() => {
    const profile = previewProfile.value
    return (
      profile?.profileName ||
      profile?.serviceProviderName ||
      profile?.profileNickname ||
      selectedProfile.value?.name ||
      ''
    )
  })
  const needsSourceImei = computed(() => selectedSource.value?.requiresSourceImei ?? false)
  const canStartTransfer = computed(() => {
    if (!selectedSource.value || !selectedProfile.value?.supported) return false
    if (!needsSourceImei.value) return true
    return sourceImei.value.trim().length > 0
  })

  const resetSelection = () => {
    profiles.value = []
    selectedProfile.value = null
    userInput.value = null
    userInputResponse.value = ''
    sourceDeletionICCID.value = ''
    previewProfile.value = null
  }

  const resetAll = () => {
    state.value = TRANSFER_STATE.idle
    stage.value = ''
    progress.value = 0
    errorMessage.value = ''
    ccidError.value = ''
    sources.value = []
    selectedSource.value = null
    sourceImei.value = ''
    resetSelection()
  }

  const setProgress = (value: number) => {
    progress.value = Math.min(Math.max(value, 0), 100)
  }

  const setStage = (nextStage: string) => {
    stage.value = nextStage
    const nextProgress = transferStageProgress[nextStage]
    if (nextProgress === undefined) return
    setProgress(nextProgress)
  }

  const session = useEsimTransferSession({
    onProgress: (nextStage) => {
      state.value = TRANSFER_STATE.progress
      setStage(nextStage)
    },
    onPreview: (profile) => {
      previewProfile.value = profile ?? previewProfile.value
      state.value = TRANSFER_STATE.progress
    },
    onUserInput: (input) => {
      userInput.value = input
      userInputResponse.value = ''
      state.value = TRANSFER_STATE.userInput
    },
    onSourceDeletion: (iccid) => {
      sourceDeletionICCID.value = iccid
      state.value = TRANSFER_STATE.sourceDeletion
    },
    onCompleted: () => {
      state.value = TRANSFER_STATE.completed
      setProgress(100)
      options?.onCompleted?.()
    },
    onError: (message) => {
      if (
        state.value === TRANSFER_STATE.completed ||
        state.value === TRANSFER_STATE.error ||
        state.value === TRANSFER_STATE.idle
      ) {
        return
      }
      state.value = TRANSFER_STATE.error
      errorMessage.value = message
    },
  })

  const loadSources = async () => {
    const targetId = modemId.value
    if (!targetId || targetId === 'unknown') return
    session.close()
    resetAll()
    state.value = TRANSFER_STATE.loadingSources
    try {
      const { data } = await esimApi.getTransferSources(targetId)
      sources.value = data.value?.sources ?? []
      ccidError.value = data.value?.ccidError ?? ''
      state.value = TRANSFER_STATE.ready
    } catch (err) {
      state.value = TRANSFER_STATE.error
      errorMessage.value = err instanceof Error ? err.message : ''
    }
  }

  const selectSource = (source: EsimTransferSource) => {
    selectedSource.value = source
    sourceImei.value = source.requiresSourceImei && modemId.value !== 'unknown' ? modemId.value : ''
    resetSelection()
  }

  const loadProfiles = async () => {
    const targetId = modemId.value
    const source = selectedSource.value
    if (!targetId || targetId === 'unknown' || !source) return
    state.value = TRANSFER_STATE.loadingProfiles
    resetSelection()
    try {
      const { data } = await esimApi.getTransferProfiles(targetId, {
        sourceType: source.type,
        sourceId: source.id,
        sourceImei: sourceImei.value.trim(),
      })
      profiles.value = data.value ?? []
      state.value = TRANSFER_STATE.ready
    } catch (err) {
      state.value = TRANSFER_STATE.error
      errorMessage.value = err instanceof Error ? err.message : ''
    }
  }

  const startTransfer = () => {
    const targetId = modemId.value
    const source = selectedSource.value
    const profile = selectedProfile.value
    if (!targetId || targetId === 'unknown' || !source || !profile?.supported) return
    session.close()
    state.value = TRANSFER_STATE.connecting
    setStage(TRANSFER_STAGE.preparing)
    errorMessage.value = ''
    previewProfile.value = null
    session.start(targetId, {
      sourceType: source.type,
      sourceId: source.id,
      profileId: profile.id,
      sourceImei: sourceImei.value.trim(),
    })
  }

  const submitUserInput = (accept: boolean) => {
    session.submitUserInput(accept, userInputResponse.value.trim())
    state.value = TRANSFER_STATE.progress
  }

  const confirmSourceDeletion = (accept: boolean) => {
    session.confirmSourceDeletion(accept)
    state.value = TRANSFER_STATE.progress
  }

  const cancelTransfer = () => {
    session.cancel()
    resetAll()
  }

  onBeforeUnmount(() => {
    session.close()
  })

  return {
    state,
    stage,
    progress,
    errorMessage,
    ccidError,
    sources,
    profiles,
    selectedSource,
    selectedProfile,
    sourceImei,
    userInput,
    userInputResponse,
    sourceDeletionICCID,
    previewProfile,
    hasSources,
    downloadedName,
    needsSourceImei,
    canStartTransfer,
    loadSources,
    selectSource,
    loadProfiles,
    startTransfer,
    submitUserInput,
    confirmSourceDeletion,
    cancelTransfer,
    resetAll,
  }
}
