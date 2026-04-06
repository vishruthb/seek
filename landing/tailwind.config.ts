import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./app/**/*.{ts,tsx}",
    "./components/**/*.{ts,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        bg: {
          primary: "var(--bg-primary)",
          secondary: "var(--bg-secondary)",
          tertiary: "var(--bg-tertiary)",
          terminal: "var(--bg-terminal)",
        },
        accent: {
          mint: "var(--accent-mint)",
          lime: "var(--accent-lime)",
          "mint-dim": "var(--accent-mint-dim)",
          glow: "var(--accent-mint-glow)",
        },
        text: {
          primary: "var(--text-primary)",
          secondary: "var(--text-secondary)",
          bright: "var(--text-bright)",
        },
        border: {
          subtle: "var(--border-subtle)",
          active: "var(--border-active)",
        },
        success: "var(--color-success)",
        warning: "var(--color-warning)",
        error: "var(--color-error)",
      },
      boxShadow: {
        terminal: "var(--shadow-terminal)",
        glow: "var(--shadow-glow)",
      },
      fontFamily: {
        mono: [
          "var(--font-jetbrains-mono)",
          "monospace",
        ],
        sans: [
          "var(--font-geist-sans)",
          "sans-serif",
        ],
      },
      maxWidth: {
        landing: "1100px",
      },
      keyframes: {
        blink: {
          "0%, 49%": { opacity: "1" },
          "50%, 100%": { opacity: "0" },
        },
        scan: {
          "0%": { transform: "translateY(-100%)" },
          "100%": { transform: "translateY(100%)" },
        },
      },
      animation: {
        blink: "blink 1s steps(1, end) infinite",
        scan: "scan 10s linear infinite",
      },
    },
  },
};

export default config;
