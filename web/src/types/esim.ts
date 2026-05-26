import type { CarrierWebsheetInfo } from '@/types/websheet'

export type EsimProfileApiResponse = {
  name: string
  serviceProviderName: string
  iccid: string
  icon: string
  profileState: number
  regionCode?: string
}

export type EsimProfilesResponse = EsimProfileApiResponse[]

export type EsimDiscoverItem = {
  eventId: string
  address: string
}

export type EsimDiscoverResponse = EsimDiscoverItem[]

export type EsimDownloadPreview = {
  iccid: string
  serviceProviderName: string
  profileName: string
  profileNickname?: string
  profileState: string
  icon?: string
  regionCode?: string
}

export type EsimProfile = {
  id: string
  name: string
  iccid: string
  enabled: boolean
  regionCode: string
  logoUrl?: string
}

export type EsimTransferSource = {
  type: 'modem' | 'ccid'
  id: string
  name: string
  detail?: string
  requiresSourceImei: boolean
}

export type EsimTransferSourcesResponse = {
  sources: EsimTransferSource[]
  ccidError?: string
}

export type EsimTransferProfile = {
  id: string
  type: 'esim' | 'physical'
  name: string
  serviceProviderName?: string
  iccid: string
  icon?: string
  regionCode?: string
  enabled: boolean
  supported: boolean
  unsupportedReason?: string
  carrierName?: string
}

export type EsimTransferWebsheet = CarrierWebsheetInfo
