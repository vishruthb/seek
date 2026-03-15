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
  title: "seek — AI-powered web search from your terminal",
  description:
    "Terminal search TUI with vim keybinds, follow-up chat, and pluggable LLM backends. Search the web without leaving your terminal.",
  metadataBase: new URL("https://vishruthb.github.io/seek/"),
  openGraph: {
    title: "seek — AI search in your terminal",
    description: "Vim keybinds. Follow-up chat. Zero context switching.",
    type: "website",
    url: "https://vishruthb.github.io/seek/",
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
