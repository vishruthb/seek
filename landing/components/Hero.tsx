"use client";

import { motion } from "framer-motion";

import CodeBlock from "@/components/ui/CodeBlock";

const installCommand = "curl -fsSL https://seekcli.vercel.app/install.sh | sh";
const githubURL = "https://github.com/vishruthb/seek";

function GitHubIcon() {
  return (
    <svg
      aria-hidden="true"
      className="h-4 w-4"
      viewBox="0 0 24 24"
      fill="currentColor"
    >
      <path d="M12 2C6.48 2 2 6.58 2 12.23c0 4.51 2.87 8.34 6.84 9.69.5.1.68-.22.68-.49 0-.24-.01-1.04-.01-1.89-2.78.62-3.37-1.21-3.37-1.21-.45-1.18-1.11-1.49-1.11-1.49-.91-.64.07-.62.07-.62 1 .07 1.53 1.06 1.53 1.06.9 1.57 2.35 1.12 2.92.86.09-.67.35-1.12.63-1.38-2.22-.26-4.56-1.14-4.56-5.09 0-1.13.39-2.05 1.03-2.77-.1-.26-.45-1.32.1-2.76 0 0 .84-.28 2.75 1.06A9.3 9.3 0 0 1 12 6.84c.85 0 1.71.12 2.51.35 1.91-1.34 2.75-1.06 2.75-1.06.55 1.44.2 2.5.1 2.76.64.72 1.03 1.64 1.03 2.77 0 3.96-2.34 4.83-4.57 5.08.36.32.67.95.67 1.92 0 1.39-.01 2.5-.01 2.84 0 .27.18.59.69.49A10.25 10.25 0 0 0 22 12.23C22 6.58 17.52 2 12 2Z" />
    </svg>
  );
}

function SeekWordmark() {
  return (
    <div className="flex items-center gap-3">
      <div className="flex h-10 w-10 items-center justify-center rounded-xl border border-border-subtle bg-bg-terminal/80 shadow-glow">
        <svg
          aria-hidden="true"
          className="h-5 w-5 text-accent-mint"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.8"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <circle cx="10.5" cy="10.5" r="5.75" />
          <path d="M15 15l4.5 4.5" />
          <path d="M8.2 10.5h4.6" />
          <path d="M10.5 8.2v4.6" />
        </svg>
      </div>
      <div className="flex flex-col">
        <span className="font-mono text-[1rem] tracking-[0.08em] text-accent-mint sm:text-[1.02rem]">
          seek
        </span>
        <span className="font-mono text-[0.68rem] uppercase tracking-[0.18em] text-text-secondary">
          cli
        </span>
      </div>
    </div>
  );
}

export default function Hero() {
  return (
    <motion.div
      className="relative flex min-w-0 w-full flex-col gap-10 lg:h-full lg:max-w-[38rem] lg:justify-between lg:gap-14"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, ease: [0.4, 0, 0.2, 1] }}
    >
      <div className="space-y-10 lg:space-y-12">
        <div className="flex w-full min-w-0 items-center justify-between gap-4">
          <SeekWordmark />
          <a
            href={githubURL}
            target="_blank"
            rel="noreferrer"
            aria-label="seek on github"
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-border-subtle text-text-primary transition-colors hover:border-accent-mint hover:text-accent-mint"
          >
            <GitHubIcon />
          </a>
        </div>
        <div className="space-y-6">
          <h1 className="max-w-2xl text-3xl font-medium tracking-tight text-text-bright sm:text-5xl xl:text-6xl">
            search the web from your terminal with your project as context.
          </h1>
          <p className="max-w-xl text-base leading-7 text-text-secondary sm:text-lg">
            seek detects your stack, attaches local files with{" "}
            <span className="font-mono text-accent-lime">@[file]</span>,
            pulls live web results, and streams a cited answer from the LLM
            you choose. one binary, fully keyboard-driven, all history saved
            locally.
          </p>
        </div>
      </div>

      <div className="max-w-2xl min-w-0 pt-2 sm:pt-4 lg:mt-auto lg:pt-4">
        <div className="max-w-2xl min-w-0">
          <CodeBlock code={installCommand} label="install" />
        </div>
      </div>

      <div className="section-shell rounded-[1.3rem] border border-border-subtle bg-bg-terminal/80 p-5 lg:hidden">
        <div className="font-mono text-sm text-accent-mint">quick start</div>
        <p className="mt-3 text-sm leading-7 text-text-secondary">
          install seek, run <span className="font-mono text-accent-lime">seek --setup</span>,
          then <span className="font-mono text-accent-lime">seek &quot;explain @[main.go]&quot;</span>{" "}
          from any repo.
        </p>
      </div>
    </motion.div>
  );
}
