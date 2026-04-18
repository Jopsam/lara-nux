import { resolve } from 'node:path';

export default defineNuxtConfig({
  ssr: false,
  devtools: {
    enabled: false,
  },
  css: ['~/assets/main.css'],
  alias: {
    '@contracts': resolve(__dirname, '../../shared/contracts/rpc/v1/contracts'),
  },
  typescript: {
    strict: true,
  },
  app: {
    head: {
      title: 'Lara Nux',
      meta: [
        {
          name: 'description',
          content: 'Desktop client for the Lara Nux local Laravel environment.',
        },
      ],
    },
  },
});
