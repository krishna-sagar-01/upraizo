// @ts-check
import { defineConfig } from 'astro/config';
import tailwindcss from '@tailwindcss/vite'
import sitemap from '@astrojs/sitemap';
import path from 'path'; 

// https://astro.build/config
export default defineConfig({
  site: 'https://upraizo.com',
  output: 'static', 
  integrations: [sitemap()],
  vite: {
    plugins: [tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve('./src'),
        '@components': path.resolve('./src/components'),
        '@common': path.resolve('./src/common'),
        '@lib': path.resolve('./src/lib'),
        '@svg': path.resolve('./src/svg'),
        '@utils': path.resolve('./src/utils'),
        '@styles': path.resolve('./src/styles')
      },
    }
  }
});