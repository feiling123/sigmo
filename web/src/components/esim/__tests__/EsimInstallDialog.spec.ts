import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import EsimInstallDialog from '@/components/esim/EsimInstallDialog.vue'
import EsimSESelector from '@/components/esim/EsimSESelector.vue'
import type { SEItem } from '@/types/se'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('vue-qrcode-reader', () => ({
  QrcodeStream: {
    template: '<div />',
  },
}))

const passthrough = {
  template: '<div><slot /></div>',
}

const formField = {
  template: '<div><slot :component-field="{}" /></div>',
}

const button = {
  props: ['disabled'],
  template: '<button type="button" :disabled="disabled"><slot /></button>',
}

const mountDialog = (allowTransfer: boolean, ses: SEItem[] = []) =>
  mount(EsimInstallDialog, {
    props: {
      open: true,
      allowTransfer,
      ses,
      'onUpdate:open': () => {},
    },
    global: {
      stubs: {
        Button: button,
        Dialog: passthrough,
        DialogDescription: passthrough,
        DialogHeader: passthrough,
        DialogTitle: passthrough,
        EsimPersistentDialogContent: passthrough,
        FormControl: passthrough,
        FormField: formField,
        FormItem: passthrough,
        FormLabel: passthrough,
        FormMessage: passthrough,
        Input: {
          template: '<input />',
        },
        RadioGroup: passthrough,
        RadioGroupItem: {
          props: ['id', 'value'],
          template: '<input :id="id" type="radio" :value="value" />',
        },
      },
    },
  })

describe('EsimInstallDialog', () => {
  it.each([
    { name: 'enabled', allowTransfer: true, wantTransfer: true },
    { name: 'disabled', allowTransfer: false, wantTransfer: false },
  ])('controls the transfer entry when $name', ({ allowTransfer, wantTransfer }) => {
    const wrapper = mountDialog(allowTransfer)

    expect(wrapper.text().includes('modemDetail.esim.transferButton')).toBe(wantTransfer)
  })

  it('disables discover and install when no SE is selected', async () => {
    const wrapper = mountDialog(false)
    await flushPromises()

    const disabledButtons = wrapper
      .findAll('button')
      .filter((button) => button.attributes('disabled') !== undefined)

    expect(disabledButtons).toHaveLength(2)
    expect(disabledButtons[0]?.attributes('aria-label')).toBe('modemDetail.esim.discover')
    expect(disabledButtons[1]?.text()).toContain('modemDetail.esim.installConfirm')
  })

  it('enables discover and install after a single SE is loaded', async () => {
    const wrapper = mountDialog(false)
    await flushPromises()

    await wrapper.setProps({
      ses: [{ id: 'default', label: 'eUICC', eid: 'eid-1' }],
    })
    await flushPromises()

    const disabledButtons = wrapper
      .findAll('button')
      .filter((button) => button.attributes('disabled') !== undefined)

    expect(disabledButtons).toHaveLength(0)
  })

  it('shows dual SE choices with EID and storage on separate lines', async () => {
    const wrapper = mountDialog(false, [
      { id: 'se0', label: 'SE1', eid: 'eid-1', freeSpace: 102400 },
      { id: 'se1', label: 'SE2', eid: 'eid-2', freeSpace: 204800 },
    ])
    await flushPromises()

    const text = wrapper.text()

    expect(text).toContain('eid-1')
    expect(text).toContain('Storage Remaining 100 KiB')
    expect(text).toContain('eid-2')
    expect(text).toContain('Storage Remaining 200 KiB')
    expect(text).not.toContain('SE1 EID')
    expect(text).not.toContain('SE2 EID')
  })

  it('does not default to the first SE for dual SE cards', async () => {
    const wrapper = mountDialog(false, [
      { id: 'se0', label: 'SE1', eid: 'eid-1', freeSpace: 102400 },
      { id: 'se1', label: 'SE2', eid: 'eid-2', freeSpace: 204800 },
    ])
    await flushPromises()

    const disabledButtons = wrapper
      .findAll('button')
      .filter((button) => button.attributes('disabled') !== undefined)

    expect(disabledButtons).toHaveLength(2)
    expect(disabledButtons[0]?.attributes('aria-label')).toBe('modemDetail.esim.discover')
    expect(disabledButtons[1]?.text()).toContain('modemDetail.esim.installConfirm')
  })

  it('clears the selected SE when a dual SE dialog reopens', async () => {
    const wrapper = mountDialog(false, [
      { id: 'se0', label: 'SE1', eid: 'eid-1', freeSpace: 102400 },
      { id: 'se1', label: 'SE2', eid: 'eid-2', freeSpace: 204800 },
    ])
    await flushPromises()

    wrapper.findComponent(EsimSESelector).vm.$emit('update:selectedSeId', 'se1')
    await flushPromises()

    await wrapper.setProps({ open: false })
    await flushPromises()
    await wrapper.setProps({ open: true })
    await flushPromises()

    const disabledButtons = wrapper
      .findAll('button')
      .filter((button) => button.attributes('disabled') !== undefined)

    expect(disabledButtons).toHaveLength(2)
    expect(disabledButtons[0]?.attributes('aria-label')).toBe('modemDetail.esim.discover')
    expect(disabledButtons[1]?.text()).toContain('modemDetail.esim.installConfirm')
  })

  it('emits the selected SE when discovery starts', async () => {
    const wrapper = mountDialog(false, [
      { id: 'se0', label: 'SE1', eid: 'eid-1', freeSpace: 102400 },
      { id: 'se1', label: 'SE2', eid: 'eid-2', freeSpace: 204800 },
    ])
    await flushPromises()

    wrapper.findComponent(EsimSESelector).vm.$emit('update:selectedSeId', 'se1')
    await flushPromises()

    const discoverButton = wrapper
      .findAll('button')
      .find((button) => button.attributes('aria-label') === 'modemDetail.esim.discover')

    expect(discoverButton).toBeDefined()
    await discoverButton?.trigger('click')

    expect(wrapper.emitted('discover')).toEqual([['se1']])
  })
})
