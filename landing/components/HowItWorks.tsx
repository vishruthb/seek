"use client";

import { motion } from "framer-motion";

const steps = [
  {
    label: "01",
    title: 'seek "your question"',
    body: "you stay in the terminal. ask the question the moment it appears.",
  },
  {
    label: "02",
    title: "tavily searches the web",
    body: "fresh pages, extracted content, and citations are gathered fast.",
  },
  {
    label: "03",
    title: "your llm synthesizes",
    body: "ollama or an openai-compatible backend reads the results and answers with citations.",
  },
];

export default function HowItWorks() {
  return (
    <section className="space-y-6">
      <div className="px-1">
        <div className="section-heading">how it works</div>
        <p className="section-copy mt-3">
          seek separates search from synthesis. tavily fetches the web results,
          then your llm reads them and writes the answer. use ollama when you
          want local answer generation, or use groq when you want speed.
        </p>
      </div>

      <motion.div
        className="section-shell rounded-[1.4rem] p-5 md:p-7"
        initial={{ opacity: 0, y: 18 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, margin: "-10% 0px" }}
        transition={{ duration: 0.2, ease: "linear" }}
      >
        <div className="flex flex-col gap-5 md:flex-row md:items-stretch">
          {steps.map((step, index) => (
            <div key={step.label} className="flex flex-1 items-stretch gap-5">
              <div className="flex-1 rounded-2xl border border-border-subtle bg-bg-secondary/90 p-5">
                <div className="mb-4 font-mono text-sm text-accent-mint">
                  {step.label}
                </div>
                <h3 className="font-mono text-lg font-semibold text-text-bright">
                  {step.title}
                </h3>
                <p className="mt-3 text-sm leading-7 text-text-secondary">
                  {step.body}
                </p>
              </div>
              {index < steps.length - 1 && (
                <div className="hidden items-center md:flex">
                  <div className="h-px w-16 border-t border-dashed border-accent-mint-dim" />
                </div>
              )}
            </div>
          ))}
        </div>
      </motion.div>
    </section>
  );
}
