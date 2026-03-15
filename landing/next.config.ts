import type { NextConfig } from "next";

const configuredBasePath = (process.env.SEEK_BASE_PATH ?? "").trim();
const normalizedBasePath =
  configuredBasePath && configuredBasePath !== "/"
    ? configuredBasePath.replace(/\/+$/, "")
    : "";

const nextConfig: NextConfig = {
  output: "export",
  ...(normalizedBasePath
    ? {
        basePath: normalizedBasePath,
        assetPrefix: `${normalizedBasePath}/`,
      }
    : {}),
  images: {
    unoptimized: true,
  },
  trailingSlash: true,
};

export default nextConfig;
