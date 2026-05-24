import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import EsimInstallDialog from '@/components/esim/EsimInstallDialog.vue'

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
  template: '<button type="button"><slot /></button>',
}

const mountDialog = (allowTransfer: boolean) =>
  mount(EsimInstallDialog, {
    props: {
      open: true,
      allowTransfer,
      'onUpdate:open': () => {},
    },
    global: {
      stubs: {
        Button: button,
        Dialog: passthrough,
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
})
