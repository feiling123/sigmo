import type { CallMediaInfo } from '@/types/call'

import {
  buildAmrBandwidthEfficientPayload,
  buildAmrOctetAlignedPayload,
  buildRtpPacket,
  parseAmrBandwidthEfficientPayload,
  parseAmrOctetAlignedPayload,
  parseRtpPacket,
  type AmrCodec,
  type AmrFrame,
  type RtpPacket,
} from './amrRtp'

export type PcmFrame = {
  samples: Float32Array<ArrayBufferLike>
  sampleRate: number
  playbackDelaySeconds?: number
}

export type AmrCodecAdapter = {
  decode(frame: AmrFrame): PcmFrame | Promise<PcmFrame>
  encode(
    samples: Float32Array<ArrayBufferLike>,
    sampleRate: number,
  ): AmrFrame[] | Promise<AmrFrame[]>
  close?: () => void | Promise<void>
}

export type CallMediaPipelineOptions = {
  media: CallMediaInfo
  codec?: AmrCodecAdapter
  onRemotePcm: (frame: PcmFrame) => void
  sendRtpPacket: (packet: Uint8Array<ArrayBuffer>) => boolean
  initialSequenceNumber?: number
  initialTimestamp?: number
  ssrc?: number
  now?: () => number
}

const defaultPTimeMillis = 20
const initialPlaybackDelaySeconds = 0.08
const minPlaybackDelaySeconds = 0.06
const maxPlaybackDelaySeconds = 0.2
const maxBufferedRemotePackets = 3
const concealmentDecay = 0.85

const random16 = () => Math.floor(Math.random() * 0x10000)
const random32 = () => Math.floor(Math.random() * 0x100000000)
const defaultNow = () => performance.now()

const normalizeAmrCodec = (value: string): AmrCodec => {
  const codec = value.trim().toUpperCase()
  if (codec === 'AMR' || codec === 'AMR-WB') {
    return codec
  }
  throw new Error(`call media codec ${value} is not supported`)
}

type PipelineCodec = AmrCodec | 'PCMU'

const normalizePipelineCodec = (value: string): PipelineCodec => {
  const codec = value.trim().toUpperCase()
  if (codec === 'PCMU') {
    return codec
  }
  return normalizeAmrCodec(codec)
}

export const resampleMono = (
  input: Float32Array<ArrayBufferLike>,
  fromRate: number,
  toRate: number,
) => {
  if (fromRate <= 0 || toRate <= 0) {
    throw new Error('sample rates must be positive')
  }
  if (fromRate === toRate) {
    return input.slice()
  }
  if (input.length === 0) {
    return new Float32Array()
  }

  const outputLength = Math.max(1, Math.round((input.length * toRate) / fromRate))
  const output = new Float32Array(outputLength)
  const scale = fromRate / toRate
  for (let i = 0; i < outputLength; i++) {
    const source = i * scale
    const left = Math.floor(source)
    const right = Math.min(left + 1, input.length - 1)
    const mix = source - left
    const leftValue = input[left] ?? 0
    const rightValue = input[right] ?? leftValue
    output[i] = leftValue * (1 - mix) + rightValue * mix
  }
  return output
}

export class CallMediaPipeline {
  readonly media: CallMediaInfo
  readonly codecName: PipelineCodec

  private readonly codec?: AmrCodecAdapter
  private readonly onRemotePcm: (frame: PcmFrame) => void
  private readonly sendRtpPacket: (packet: Uint8Array<ArrayBuffer>) => boolean
  private readonly now: () => number
  private readonly samplesPerPacket: number
  private sequenceNumber: number
  private timestamp: number
  private readonly ssrc: number
  private localBuffer: Float32Array<ArrayBufferLike> = new Float32Array()
  private sentFirstPacket = false
  private expectedRemoteSequenceNumber: number | null = null
  private remotePackets = new Map<number, RtpPacket>()
  private previousRemoteTransit: number | null = null
  private remoteJitter = 0
  private remotePlaybackDelaySeconds = initialPlaybackDelaySeconds
  private lastRemoteFrame: PcmFrame | null = null
  private remoteFlushPromise: Promise<void> = Promise.resolve()

  constructor(options: CallMediaPipelineOptions) {
    this.media = options.media
    this.codecName = normalizePipelineCodec(options.media.codec)
    this.codec = options.codec
    this.onRemotePcm = options.onRemotePcm
    this.sendRtpPacket = options.sendRtpPacket
    this.now = options.now ?? defaultNow
    this.samplesPerPacket = Math.max(
      1,
      Math.round(
        (options.media.clockRate * (options.media.ptimeMillis || defaultPTimeMillis)) / 1000,
      ),
    )
    this.sequenceNumber = options.initialSequenceNumber ?? random16()
    this.timestamp = options.initialTimestamp ?? random32()
    this.ssrc = options.ssrc ?? random32()
  }

  async receiveRtpPacket(packet: ArrayBuffer | Uint8Array<ArrayBuffer>) {
    const rtp = parseRtpPacket(packet)
    if (rtp.payloadType !== this.media.payloadType) {
      return false
    }

    this.updateRemoteJitter(rtp)
    this.queueRemotePacket(rtp)
    const flush = this.remoteFlushPromise.then(() => this.flushRemotePackets())
    this.remoteFlushPromise = flush.catch(() => {})
    await flush
    return true
  }

  private updateRemoteJitter(rtp: RtpPacket) {
    const arrival = (this.now() * this.media.clockRate) / 1000
    const transit = arrival - rtp.timestamp
    if (this.previousRemoteTransit !== null) {
      const delta = Math.abs(transit - this.previousRemoteTransit)
      this.remoteJitter += (delta - this.remoteJitter) / 16
      const target = clamp(
        initialPlaybackDelaySeconds + (this.remoteJitter / this.media.clockRate) * 2,
        minPlaybackDelaySeconds,
        maxPlaybackDelaySeconds,
      )
      this.remotePlaybackDelaySeconds =
        target > this.remotePlaybackDelaySeconds
          ? target
          : this.remotePlaybackDelaySeconds * 0.98 + target * 0.02
    }
    this.previousRemoteTransit = transit
  }

  private queueRemotePacket(rtp: RtpPacket) {
    if (
      this.expectedRemoteSequenceNumber !== null &&
      !sequenceAheadOrEqual(rtp.sequenceNumber, this.expectedRemoteSequenceNumber)
    ) {
      return
    }
    if (this.remotePackets.has(rtp.sequenceNumber)) {
      return
    }
    this.remotePackets.set(rtp.sequenceNumber, rtp)
    this.expectedRemoteSequenceNumber ??= rtp.sequenceNumber
  }

  private async flushRemotePackets() {
    while (this.expectedRemoteSequenceNumber !== null) {
      const packet = this.remotePackets.get(this.expectedRemoteSequenceNumber)
      if (!packet) {
        if (this.remotePackets.size < maxBufferedRemotePackets) {
          return
        }
        this.emitRemotePcm(this.concealRemoteFrame())
        this.expectedRemoteSequenceNumber = nextSequenceNumber(this.expectedRemoteSequenceNumber)
        continue
      }
      this.remotePackets.delete(this.expectedRemoteSequenceNumber)
      await this.decodeRemotePacket(packet)
      this.expectedRemoteSequenceNumber = nextSequenceNumber(this.expectedRemoteSequenceNumber)
    }
  }

  private async decodeRemotePacket(rtp: RtpPacket) {
    if (this.codecName === 'PCMU') {
      this.emitRemotePcm({ samples: decodePCMU(rtp.payload), sampleRate: this.media.clockRate })
      return
    }

    if (!this.codec) {
      throw new Error(`${this.codecName} codec is not available`)
    }
    const payload = this.media.octetAlign
      ? parseAmrOctetAlignedPayload(this.codecName, rtp.payload)
      : parseAmrBandwidthEfficientPayload(this.codecName, rtp.payload)
    for (const frame of payload.frames) {
      if (!frame.quality || frame.data.byteLength === 0) {
        continue
      }
      this.emitRemotePcm(await this.codec.decode(frame))
    }
  }

  private emitRemotePcm(frame: PcmFrame) {
    const next = {
      ...frame,
      playbackDelaySeconds: this.remotePlaybackDelaySeconds,
    }
    this.lastRemoteFrame = {
      samples: next.samples.slice(),
      sampleRate: next.sampleRate,
      playbackDelaySeconds: next.playbackDelaySeconds,
    }
    this.onRemotePcm(next)
  }

  private concealRemoteFrame(): PcmFrame {
    if (!this.lastRemoteFrame || this.lastRemoteFrame.samples.length === 0) {
      return {
        samples: new Float32Array(this.samplesPerPacket),
        sampleRate: this.media.clockRate,
      }
    }
    const samples = new Float32Array(this.lastRemoteFrame.samples.length)
    for (let i = 0; i < samples.length; i++) {
      samples[i] = (this.lastRemoteFrame.samples[i] ?? 0) * concealmentDecay
    }
    return {
      samples,
      sampleRate: this.lastRemoteFrame.sampleRate,
    }
  }

  async sendPcm(samples: Float32Array<ArrayBufferLike>, sampleRate: number) {
    const converted = resampleMono(samples, sampleRate, this.media.clockRate)
    this.localBuffer = appendSamples(this.localBuffer, converted)

    let sent = 0
    while (this.localBuffer.length >= this.samplesPerPacket) {
      const chunk = this.localBuffer.slice(0, this.samplesPerPacket)
      this.localBuffer = this.localBuffer.slice(this.samplesPerPacket)
      let payload: Uint8Array<ArrayBuffer>
      if (this.codecName === 'PCMU') {
        payload = encodePCMU(chunk)
      } else {
        if (!this.codec) {
          throw new Error(`${this.codecName} codec is not available`)
        }
        const frames = await this.codec.encode(chunk, this.media.clockRate)
        if (frames.length === 0) {
          continue
        }
        payload = this.media.octetAlign
          ? buildAmrOctetAlignedPayload(this.codecName, frames)
          : buildAmrBandwidthEfficientPayload(this.codecName, frames)
      }
      const packet = buildRtpPacket({
        payloadType: this.media.payloadType,
        sequenceNumber: this.sequenceNumber,
        timestamp: this.timestamp,
        ssrc: this.ssrc,
        marker: !this.sentFirstPacket,
        payload,
      })

      if (!this.sendRtpPacket(packet)) {
        return sent
      }
      this.sentFirstPacket = true
      this.sequenceNumber = (this.sequenceNumber + 1) & 0xffff
      this.timestamp = (this.timestamp + this.samplesPerPacket) >>> 0
      sent++
    }
    return sent
  }

  close() {
    this.localBuffer = new Float32Array()
    this.remotePackets.clear()
    this.expectedRemoteSequenceNumber = null
    this.previousRemoteTransit = null
    this.remoteJitter = 0
    this.remotePlaybackDelaySeconds = initialPlaybackDelaySeconds
    this.lastRemoteFrame = null
    this.remoteFlushPromise = Promise.resolve()
    void this.codec?.close?.()
  }
}

const nextSequenceNumber = (value: number) => (value + 1) & 0xffff

const sequenceAheadOrEqual = (left: number, right: number) => ((left - right) & 0xffff) < 0x8000

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value))

export const decodePCMU = (payload: Uint8Array<ArrayBufferLike>) => {
  const out = new Float32Array(payload.byteLength)
  for (let i = 0; i < payload.byteLength; i++) {
    const value = ~payload[i] & 0xff
    const sign = value & 0x80
    const exponent = (value >> 4) & 0x07
    const mantissa = value & 0x0f
    const sample = (((mantissa << 3) + 0x84) << exponent) - 0x84
    out[i] = (sign ? -sample : sample) / 32768
  }
  return out
}

export const encodePCMU = (samples: Float32Array<ArrayBufferLike>) => {
  const out = new Uint8Array(samples.length)
  for (let i = 0; i < samples.length; i++) {
    let sample = Math.round(Math.max(-1, Math.min(1, samples[i] ?? 0)) * 32767)
    let sign = 0
    if (sample < 0) {
      sign = 0x80
      sample = -sample
    }
    sample = Math.min(sample, 32635) + 0x84

    let exponent = 7
    for (let mask = 0x4000; exponent > 0 && (sample & mask) === 0; mask >>= 1) {
      exponent--
    }
    const mantissa = (sample >> (exponent + 3)) & 0x0f
    out[i] = ~(sign | (exponent << 4) | mantissa) & 0xff
  }
  return out
}

const appendSamples = (
  left: Float32Array<ArrayBufferLike>,
  right: Float32Array<ArrayBufferLike>,
) => {
  if (left.length === 0) {
    return right
  }
  if (right.length === 0) {
    return left
  }
  const out = new Float32Array(left.length + right.length)
  out.set(left)
  out.set(right, left.length)
  return out
}
