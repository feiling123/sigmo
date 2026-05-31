# Sigmo

This template should help get you started developing with Vue 3 in Vite.

## Recommended IDE Setup

[VS Code](https://code.visualstudio.com/) + [Vue (Official)](https://marketplace.visualstudio.com/items?itemName=Vue.volar) (and disable Vetur).

## Recommended Browser Setup

- Chromium-based browsers (Chrome, Edge, Brave, etc.):
  - [Vue.js devtools](https://chromewebstore.google.com/detail/vuejs-devtools/nhdogjmejiglipccpnnnanhbledajbpd)
  - [Turn on Custom Object Formatter in Chrome DevTools](http://bit.ly/object-formatters)
- Firefox:
  - [Vue.js devtools](https://addons.mozilla.org/en-US/firefox/addon/vue-js-devtools/)
  - [Turn on Custom Object Formatter in Firefox DevTools](https://fxdx.dev/firefox-devtools-custom-object-formatters/)

## Type Support for `.vue` Imports in TS

TypeScript cannot handle type information for `.vue` imports by default, so we replace the `tsc` CLI with `vue-tsc` for type checking. In editors, we need [Volar](https://marketplace.visualstudio.com/items?itemName=Vue.volar) to make the TypeScript language service aware of `.vue` types.

## Customize configuration

See [Vite Configuration Reference](https://vite.dev/config/).

## Project Setup

```sh
bun install
```

## Voice Media

Sigmo uses browser WebRTC for call audio. The backend transcodes carrier
AMR/AMR-WB RTP to PCMU for the browser and encodes browser PCMU audio back to
the negotiated carrier AMR format. Build the service-side AMR WebAssembly codec
with:

```sh
scripts/build-opencore-amr-wasi.sh
```

Set `SIGMO_AMR_WASM` when the codec is not available at the default path
`internal/pkg/voicecodec/assets/opencore-amr.wasm`.

The browser and backend use one built-in Google STUN server and one Cloudflare
STUN server for ICE candidate discovery. There is no runtime STUN configuration.

The backend WebRTC ICE UDP ports are pinned to `40000-40100`; expose that UDP
range on the server firewall.

### Compile and Hot-Reload for Development

```sh
bun dev
```

### Type-Check, Compile and Minify for Production

```sh
bun run build
```

### Run Unit Tests with [Vitest](https://vitest.dev/)

```sh
bun test:unit
```

### Lint with [ESLint](https://eslint.org/)

```sh
bun lint
```
