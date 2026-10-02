import vue from "@vitejs/plugin-vue"
import { defineConfig } from "vitest/config"

export default defineConfig({
  clearScreen: false,

  // The Go server serves the build below /assets/ and, in development,
  // proxies /assets/ to this dev server.
  base:    "/assets/",
  plugins: [vue()],
  define:  {
    __VUE_I18N_FULL_INSTALL__: true,
    __VUE_I18N_LEGACY_API__:   false,
    __INTLIFY_PROD_DEVTOOLS__: false,
  },
  build: {
    outDir:        "dist/app",
    assetsDir:     "",
    manifest:      true,
    rollupOptions: {
      input: {
        public: "src/public/main.ts",
        admin:  "src/admin/main.ts",
      },
    },
  },
  server: {
    port:       5173,
    strictPort: true,
    fs:         { allow: [".."] },
  },
  css: {
    // Bulma triggers deprecation warnings of newer Sass releases.
    preprocessorOptions: { scss: { quietDeps: true } },
  },
  test: {
    environment: "happy-dom",
    include:     ["src/**/*.test.ts"],
  },
})
