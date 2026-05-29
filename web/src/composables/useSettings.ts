import { ref } from 'vue'

import { useSettingsApi } from '@/apis/settings'
import type { SettingsResponse, SettingsValues } from '@/types/settings'

const cloneValues = (values: SettingsValues): SettingsValues => {
  return JSON.parse(JSON.stringify(values)) as SettingsValues
}

export const useSettings = () => {
  const configApi = useSettingsApi()

  const settings = ref<SettingsResponse | null>(null)
  const values = ref<SettingsValues | null>(null)
  const isLoading = ref(false)
  const isSaving = ref(false)

  const fetchSettings = async () => {
    if (isLoading.value) return
    isLoading.value = true
    try {
      const { data } = await configApi.getSettings()
      if (!data.value) return
      settings.value = data.value
      values.value = cloneValues(data.value.values)
    } finally {
      isLoading.value = false
    }
  }

  const saveSettings = async () => {
    if (!values.value || isSaving.value) return null
    isSaving.value = true
    try {
      const { data } = await configApi.updateSettings(values.value)
      if (!data.value) return null
      settings.value = data.value
      values.value = cloneValues(data.value.values)
      return data.value
    } finally {
      isSaving.value = false
    }
  }

  return {
    settings,
    values,
    isLoading,
    isSaving,
    fetchSettings,
    saveSettings,
  }
}
