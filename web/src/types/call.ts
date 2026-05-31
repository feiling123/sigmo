export type CallRoute = 'auto' | 'wifi_calling' | 'modem'
export type StoredCallRoute = Exclude<CallRoute, 'auto'>
export type CallDirection = 'incoming' | 'outgoing'
export type CallState =
  | 'dialing'
  | 'ringing'
  | 'answering'
  | 'early_media'
  | 'active'
  | 'confirmed'
  | 'ending'
  | 'ended'
  | 'failed'

export type CallRecord = {
  callID: string
  route: StoredCallRoute
  direction: CallDirection
  number: string
  state: CallState
  reason: string
  startedAt: string
  answeredAt: string
  endedAt: string
  updatedAt: string
}

export type DialCallRequest = {
  to: string
  route: CallRoute
}

export type UpdateCallRequest = {
  state: 'active' | 'ended'
  reason?: 'busy' | ''
}

export type CallEventMessage = {
  type: 'call'
  call: CallRecord
}

export type WebRTCSessionDescriptionPayload = {
  type: 'offer' | 'answer'
  sdp: string
}
