import { fetchJson } from '@/lib/fetch'

import type { SettingsResponse, SettingsValues } from '@/types/settings'

export const useSettingsApi = () => {
  const getSettings = () => {
    return fetchJson<SettingsResponse>('settings')
  }

  const updateSettings = (payload: SettingsValues) => {
    return fetchJson<SettingsResponse>('settings', {
      method: 'PUT',
      body: JSON.stringify(payload),
    })
  }

  return {
    getSettings,
    updateSettings,
  }
}
