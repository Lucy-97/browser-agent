import type { NextConfig } from "next";

const GO_API_BASE = process.env.GO_API_BASE_URL || "http://127.0.0.1:29001";

const nextConfig: NextConfig = {
  output: "standalone",
  allowedDevOrigins: ["127.0.0.1"],
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
        source: "/api/v1/:path*",
        destination: `${GO_API_BASE}/api/v1/:path*`,
      },
      {
        source: "/api/:path*",
        destination: `${GO_API_BASE}/:path*`,
      },
    ];
  },
};

export default nextConfig;
