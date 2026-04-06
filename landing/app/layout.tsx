import type { Metadata } from "next";
import { Analytics } from "@vercel/analytics/next";
import { Geist, JetBrains_Mono } from "next/font/google";

import "./globals.css";

const themeInitScript = `
(() => {
  const faviconHrefFor = (theme) => theme === "light" ? "/favicon-light.svg" : "/favicon-dark.svg";
  const applyTheme = (theme) => {
    document.documentElement.dataset.theme = theme;
    const href = faviconHrefFor(theme);
    document.querySelectorAll('link[rel*="icon"]').forEach((node) => {
      node.setAttribute("href", href);
    });
  };
  try {
    const stored = window.localStorage.getItem("seek-landing-theme");
    const resolved = stored === "light" || stored === "dark"
      ? stored
      : (window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark");
    applyTheme(resolved);
  } catch {
    applyTheme("dark");
  }
})();
`;

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
  icons: {
    icon: "/favicon-dark.svg",
    shortcut: "/favicon-dark.svg",
    apple: "/favicon-dark.svg",
  },
  openGraph: {
    title: "seek - project-aware search from your terminal",
    description: "Project context. @[file] attachments. Local history. Zero browser tab drift.",
    type: "website",
    url: "https://seekcli.vercel.app/",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
      </head>
      <body
        className={`${geistSans.variable} ${jetbrainsMono.variable} min-h-screen bg-bg-primary font-sans text-text-primary antialiased`}
      >
        {children}
        <Analytics />
      </body>
    </html>
  );
}
