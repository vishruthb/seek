"use client";

import { motion } from "framer-motion";

import GlowCard from "@/components/ui/GlowCard";

const features = [
  {
    icon: "»",
    title: "search without switching",
    text: "fast web search from your terminal. results stream in real time with markdown rendering.",
  },
  {
    icon: "jk",
    title: "vim keybinds",
    text: "j/k to scroll, tab to switch panels, / to search within results, y to yank.",
  },
  {
    icon: "f>",
    title: "follow-up chat",
    text: "press f to ask follow-ups. context is preserved across the whole session.",
  },
  {
    icon: "llm",
    title: "pluggable backends",
    text: "ollama for local answer generation. groq, openrouter, and together for cloud. swap mid-session with /backend.",
  },
  {
    icon: "Y",
    title: "yank code blocks",
    text: "press Y to copy code blocks to the clipboard. multiple blocks? pick by number.",
  },
  {
    icon: "/",
    title: "configurable modes",
    text: "concise, learning, explanatory, or oneliner. switch with /mode anytime.",
  },
];

export default function Features() {
  return (
    <section className="space-y-6">
      <div className="px-1">
        <div className="section-heading">features</div>
        <p className="section-copy mt-3">
          it's built for the moment you need an answer now, not after a browser
          detour. one binary, one terminal, one flow.
        </p>
      </div>

      <motion.div
        className="grid gap-4 md:grid-cols-2 xl:grid-cols-3"
        initial="hidden"
        whileInView="visible"
        viewport={{ once: true, margin: "-10% 0px" }}
        variants={{
          hidden: {},
          visible: {
            transition: {
              staggerChildren: 0.08,
            },
          },
        }}
      >
        {features.map((feature) => (
          <motion.div
            key={feature.title}
            variants={{
              hidden: { opacity: 0, y: 20 },
              visible: { opacity: 1, y: 0 },
            }}
            transition={{ duration: 0.16, ease: "linear" }}
          >
            <GlowCard className="h-full">
              <div className="flex items-start gap-4">
                <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-border-subtle bg-bg-tertiary font-mono text-sm font-semibold text-accent-mint">
                  {feature.icon}
                </div>
                <div className="space-y-2">
                  <h3 className="font-mono text-lg font-semibold text-text-bright">
                    {feature.title}
                  </h3>
                  <p className="text-sm leading-7 text-text-secondary">
                    {feature.text}
                  </p>
                </div>
              </div>
            </GlowCard>
          </motion.div>
        ))}
      </motion.div>
    </section>
  );
}
