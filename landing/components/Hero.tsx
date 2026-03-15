"use client";

import { motion } from "framer-motion";

import CodeBlock from "@/components/ui/CodeBlock";

const installCommand = "curl -fsSL https://vishruthb.github.io/seek/install.sh | sh";

export default function Hero() {
  return (
    <motion.div
      className="relative flex w-full flex-col gap-6 lg:max-w-[38rem]"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.24, ease: [0.4, 0, 0.2, 1] }}
    >
      <div className="font-mono text-[0.78rem] text-accent-mint">seek cli</div>
      <div className="space-y-5">
        <h1 className="max-w-2xl text-3xl font-medium tracking-tight text-text-bright sm:text-5xl xl:text-6xl">
          ai-powered web search, all from the comfort of your terminal.
        </h1>
        <p className="max-w-xl text-base leading-7 text-text-secondary sm:text-lg">
          fast, keyboard-first, and lightweight. use ollama when you want local
          answer generation, or switch to groq and other openai-compatible
          backends when you want raw speed.
        </p>
      </div>

      <div className="max-w-2xl pt-3 sm:pt-5">
        <div className="max-w-2xl">
          <CodeBlock code={installCommand} label="install" />
        </div>
      </div>

      <div className="section-shell rounded-[1.3rem] border border-border-subtle bg-bg-terminal/80 p-5 lg:hidden">
        <div className="font-mono text-sm text-accent-mint">desktop interactive demo</div>
        <p className="mt-3 text-sm leading-7 text-text-secondary">
          the live terminal demo shows up on wider screens. on a phone, install
          seek and try it in your own terminal.
        </p>
      </div>
    </motion.div>
  );
}
