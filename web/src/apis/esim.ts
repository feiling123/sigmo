import { useFetch } from '@/lib/fetch'

import type {
  EsimDiscoverResponse,
  EsimProfilesResponse,
  EsimTransferProfile,
  EsimTransferSourcesResponse,
} from '@/types/esim'

export const useEsimApi = () => {
  const getEsims = (id: string) => {
    return useFetch<EsimProfilesResponse>(`modems/${id}/esims`).get().json()
  }

  const discoverEsims = (id: string) => {
    return useFetch<EsimDiscoverResponse>(`modems/${id}/esim-discoveries`, {
      method: 'POST',
    }).json()
  }

  const updateEsimNickname = (id: string, iccid: string, nickname: string) => {
    return useFetch<void>(`modems/${id}/esims/${iccid}/nickname`, {
      method: 'PUT',
      body: JSON.stringify({ nickname }),
    }).json()
  }

  const enableEsim = (id: string, iccid: string) => {
    return useFetch<void>(`modems/${id}/esims/${iccid}/activation`, {
      method: 'PUT',
    }).json()
  }

  const deleteEsim = (id: string, iccid: string) => {
    return useFetch<void>(`modems/${id}/esims/${iccid}`, {
      method: 'DELETE',
    }).json()
  }

  const getTransferSources = (id: string) => {
    return useFetch<EsimTransferSourcesResponse>(`modems/${id}/esim-transfer-sources`).get().json()
  }

  const getTransferProfiles = (
    id: string,
    payload: { sourceType: string; sourceId: string; sourceImei?: string },
  ) => {
    return useFetch<EsimTransferProfile[]>(`modems/${id}/esim-transfer-profile-queries`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }).json()
  }

  return {
    getEsims,
    discoverEsims,
    updateEsimNickname,
    enableEsim,
    deleteEsim,
    getTransferSources,
    getTransferProfiles,
  }
}
