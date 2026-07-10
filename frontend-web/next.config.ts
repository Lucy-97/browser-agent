import type { NextConfig } from "next";

const GO_API_BASE = process.env.GO_API_BASE_URL || "http://127.0.0.1:28001";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/web/:path*",
        destination: `${GO_API_BASE}/web/:path*`,
      },
      {
        source: "/worker/:path*",
        destination: `${GO_API_BASE}/worker/:path*`,
      },
      {
        source: "/admin/:path*",
        destination: `${GO_API_BASE}/admin/:path*`,
      },
      {
        source: "/api/:path*",
        destination: `${GO_API_BASE}/:path*`,
      },
    ];
  },
};

export default nextConfig;
