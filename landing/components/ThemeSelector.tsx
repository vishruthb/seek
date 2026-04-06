"use client";

import { useEffect, useState } from "react";

type ThemeName = "dark" | "light";

const storageKey = "seek-landing-theme";
const options: ThemeName[] = ["dark", "light"];

function faviconHrefFor(theme: ThemeName) {
  return theme === "light" ? "/favicon-light.svg" : "/favicon-dark.svg";
}

function applyTheme(theme: ThemeName) {
  document.documentElement.dataset.theme = theme;
  const href = faviconHrefFor(theme);
  document.querySelectorAll('link[rel*="icon"]').forEach((node) => {
    node.setAttribute("href", href);
  });
}

function MoonIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M21 12.8A8.8 8.8 0 1 1 11.2 3a7.1 7.1 0 0 0 9.8 9.8Z" />
    </svg>
  );
}

function SunIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.9"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="12" cy="12" r="4.2" />
      <path d="M12 2.5v2.2" />
      <path d="M12 19.3v2.2" />
      <path d="m4.9 4.9 1.6 1.6" />
      <path d="m17.5 17.5 1.6 1.6" />
      <path d="M2.5 12h2.2" />
      <path d="M19.3 12h2.2" />
      <path d="m4.9 19.1 1.6-1.6" />
      <path d="m17.5 6.5 1.6-1.6" />
    </svg>
  );
}

function resolveTheme(): ThemeName {
  if (typeof document !== "undefined") {
    const current = document.documentElement.dataset.theme;
    if (current === "dark" || current === "light") {
      return current;
    }
  }
  if (typeof window !== "undefined" && window.matchMedia("(prefers-color-scheme: light)").matches) {
    return "light";
  }
  return "dark";
}

export default function ThemeSelector() {
  const [theme, setTheme] = useState<ThemeName>("dark");

  useEffect(() => {
    const nextTheme = resolveTheme();
    setTheme(nextTheme);
    applyTheme(nextTheme);
  }, []);

  function selectTheme(nextTheme: ThemeName) {
    setTheme(nextTheme);
    applyTheme(nextTheme);
    try {
      window.localStorage.setItem(storageKey, nextTheme);
    } catch {}
  }

  return (
    <div
      className="relative inline-grid grid-cols-2 rounded-full border border-border-subtle bg-bg-secondary/90 p-1 shadow-glow"
      role="group"
      aria-label="theme selector"
    >
      <span
        aria-hidden="true"
        className={`pointer-events-none absolute top-1 left-1 h-9 w-9 rounded-full border border-border-active bg-accent-glow shadow-[0_0_0_1px_var(--shell-inset)_inset] transition-transform duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] ${
          theme === "light" ? "translate-x-9" : "translate-x-0"
        }`}
      />
      {options.map((option) => {
        const active = option === theme;
        return (
          <button
            key={option}
            type="button"
            onClick={() => selectTheme(option)}
            aria-pressed={active}
            aria-label={`Switch to ${option} mode`}
            title={option === "dark" ? "Dark mode" : "Light mode"}
            className={`relative z-10 inline-flex h-9 w-9 items-center justify-center rounded-full border border-transparent transition-[color,transform] duration-300 ease-[cubic-bezier(0.22,1,0.36,1)] ${
              active
                ? "text-text-bright"
                : "text-text-secondary hover:text-text-bright"
            }`}
          >
            {option === "dark" ? <MoonIcon /> : <SunIcon />}
          </button>
        );
      })}
    </div>
  );
}
