import { useFetch } from '@/lib/fetch'

import type { CapabilitiesResponse } from '@/types/capability'

export const useCapabilityApi = () => {
  const getCapabilities = () => {
    return useFetch<CapabilitiesResponse>('capabilities').get().json()
  }

  return {
    getCapabilities,
  }
}
