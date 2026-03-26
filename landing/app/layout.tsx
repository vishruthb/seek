import type { Metadata } from "next";
import { Geist, JetBrains_Mono } from "next/font/google";

import "./globals.css";

const geistSans = Geist({
  subsets: ["latin"],
  variable: "--font-geist-sans",
});

const jetbrainsMono = JetBrains_Mono({
  subsets: ["latin"],
  variable: "--font-jetbrains-mono",
});

export const metadata: Metadata = {
  title: "seek - project-aware search from your terminal",
  description:
    "Terminal search TUI with project context detection, local file attachments, saved history, latency metrics, and pluggable LLM backends.",
  metadataBase: new URL("https://seekcli.vercel.app/"),
  openGraph: {
    title: "seek - project-aware search from your terminal",
    description: "Project context. @[file] attachments. Local history. Zero browser tab drift.",
    type: "website",
    url: "https://seekcli.vercel.app/",
    images: ["/og-image.png"],
  },
  twitter: {
    card: "summary_large_image",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body
        className={`${geistSans.variable} ${jetbrainsMono.variable} min-h-screen bg-bg-primary font-sans text-text-primary antialiased`}
      >
        {children}
      </body>
    </html>
  );
}
