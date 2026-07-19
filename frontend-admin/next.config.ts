import type { NextConfig } from "next";

const GO_API_BASE = process.env.GO_API_BASE_URL || "http://127.0.0.1:28001";

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
        source: "/api/:path*",
        destination: `${GO_API_BASE}/:path*`, 
      },
    ];
  },
};

export default nextConfig;
