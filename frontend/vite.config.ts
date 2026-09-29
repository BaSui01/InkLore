import { defineConfig, loadEnv } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import { resolve } from "path"

export default defineConfig(({ mode }) => {
  const rootDir = resolve(__dirname, "..")
  const env = loadEnv(mode, rootDir, "")
  const creatorPort = parseInt(env.VITE_CREATOR_PORT || "3000", 10)
  const backendUrl = env.VITE_BACKEND_URL || "http://127.0.0.1:8080"

  const configureWsProxy = (proxy: any) => {
    const isIgnorable = (err: any) => {
      const code = err?.code
      const msg = String(err?.message || "")
      return (
        code === "ECONNABORTED" ||
        code === "ECONNRESET" ||
        code === "EPIPE" ||
        code === "ETIMEDOUT" ||
        msg.includes("ECONNABORTED") ||
        msg.includes("ECONNRESET") ||
        msg.includes("EPIPE")
      )
    }

    const originalOn = proxy.on.bind(proxy)
    proxy.on = function (event: string, listener: any) {
      if (event === "error") {
        return originalOn("error", (err: any, req: any, res: any) => {
          if (isIgnorable(err)) {
            if (res && !("req" in res) && typeof res.end === "function") {
              try {
                res.end()
              } catch {}
            }
            return
          }
          return listener(err, req, res)
        })
      }
      if (event === "proxyReqWs") {
        return originalOn("proxyReqWs", (proxyReq: any, req: any, socket: any, options: any, head: any) => {
          const origSocketOn = socket.on.bind(socket)
          socket.on = function (sEvent: string, sListener: any) {
            if (sEvent === "error") {
              return origSocketOn("error", (err: any) => {
                if (isIgnorable(err)) return
                return sListener(err)
              })
            }
            return origSocketOn(sEvent, sListener)
          }
          return listener(proxyReq, req, socket, options, head)
        })
      }
      return originalOn(event, listener)
    }
  }

  return {
    appType: "spa",
    envDir: rootDir,
    cacheDir: resolve(__dirname, "node_modules/.vite-creator"),
    optimizeDeps: {
      include: [
        "react",
        "react/jsx-runtime",
        "react/jsx-dev-runtime",
        "react-dom",
        "react-dom/client",
        "@tanstack/react-query",
        "cytoscape",
        "@tiptap/react",
        "@tiptap/starter-kit",
        "@tiptap/extension-placeholder",
        "@tiptap/extension-character-count",
        "@tiptap/extension-bubble-menu",
        "clsx",
        "tailwind-merge",
        "lucide-react",
      ],
    },
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        "@inklore/contracts": resolve(__dirname, "packages/contracts/src"),
        "@inklore/transport": resolve(__dirname, "packages/transport/src"),
        "@inklore/recovery": resolve(__dirname, "packages/recovery/src"),
        "@inklore/i18n": resolve(__dirname, "packages/i18n/src"),
        "@inklore/ui": resolve(__dirname, "packages/ui/src"),
        "@inklore/run-ui": resolve(__dirname, "packages/run-ui/src"),
      },
    },
    server: {
      host: "127.0.0.1",
      port: creatorPort,
      watch: {
        ignored: ["**/dist/**", "**/dist-admin/**"],
      },
      proxy: {
        "/v1": {
          target: backendUrl,
          changeOrigin: true,
          ws: true,
          rewriteWsOrigin: true,
          configure: configureWsProxy,
        },
        "/healthz": backendUrl,
      },
    },
    preview: {
      port: creatorPort,
      proxy: {
        "/v1": {
          target: backendUrl,
          changeOrigin: true,
          ws: true,
          rewriteWsOrigin: true,
          configure: configureWsProxy,
        },
        "/healthz": backendUrl,
      },
    },
    build: {
      outDir: "dist",
      chunkSizeWarningLimit: 1200,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.includes("node_modules")) {
              if (id.includes("@tiptap")) {
                return "vendor-tiptap"
              }
              if (id.includes("lucide-react")) {
                return "vendor-icons"
              }
              if (id.includes("cytoscape")) {
                return "vendor-graph"
              }
              return "vendor-core"
            }
          },
        },
      },
    },
  }
})
