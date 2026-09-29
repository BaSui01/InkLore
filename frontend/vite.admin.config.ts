import { defineConfig, loadEnv } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"
import { resolve } from "path"

export default defineConfig(({ mode }) => {
  const rootDir = resolve(__dirname, "..")
  const env = loadEnv(mode, rootDir, "")
  const adminPort = parseInt(env.VITE_ADMIN_PORT || "3001", 10)
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
    envDir: rootDir,
    cacheDir: resolve(__dirname, "node_modules/.vite-admin"),
    optimizeDeps: {
      include: [
        "react",
        "react/jsx-runtime",
        "react/jsx-dev-runtime",
        "react-dom",
        "react-dom/client",
        "clsx",
        "tailwind-merge",
        "lucide-react",
      ],
    },
    plugins: [
      react(),
      tailwindcss(),
      {
        name: "admin-index-rewrite",
        configureServer(server) {
          server.middlewares.use((req, _res, next) => {
            const rawUrl = req.url || "/";
            const pathname = rawUrl.split("?")[0];
            const search = rawUrl.includes("?") ? rawUrl.slice(rawUrl.indexOf("?")) : "";
            if (
              !pathname.startsWith("/v1") &&
              !pathname.startsWith("/healthz") &&
              !pathname.startsWith("/@") &&
              !pathname.startsWith("/src") &&
              !pathname.startsWith("/packages") &&
              !pathname.startsWith("/node_modules") &&
              !pathname.includes(".")
            ) {
              req.url = "/admin.html" + search;
            } else if (pathname === "/" || pathname === "/index.html") {
              req.url = "/admin.html" + search;
            }
            next();
          });
        },
        configurePreviewServer(server) {
          server.middlewares.use((req, _res, next) => {
            const rawUrl = req.url || "/";
            const pathname = rawUrl.split("?")[0];
            const search = rawUrl.includes("?") ? rawUrl.slice(rawUrl.indexOf("?")) : "";
            if (
              !pathname.startsWith("/v1") &&
              !pathname.startsWith("/healthz") &&
              !pathname.includes(".")
            ) {
              req.url = "/admin.html" + search;
            }
            next();
          });
        },
      },
    ],
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
      port: adminPort,
      open: true,
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
      port: adminPort,
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
      outDir: "dist-admin",
      chunkSizeWarningLimit: 1200,
      rollupOptions: {
        input: {
          admin: resolve(__dirname, "admin.html"),
        },
        output: {
          manualChunks(id) {
            if (id.includes("node_modules")) {
              if (id.includes("lucide-react")) {
                return "vendor-icons"
              }
              return "vendor-core"
            }
          },
        },
      },
    },
  }
})
