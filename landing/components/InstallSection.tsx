"use client";

import { useState } from "react";

import { motion } from "framer-motion";

import CodeBlock from "@/components/ui/CodeBlock";

const installCommand = "curl -fsSL https://seekcli.vercel.app/install.sh | sh";

const tabs = [
  {
    id: "shell",
    label: "shell script",
    code: installCommand,
  },
  {
    id: "verify",
    label: "verify install",
    code: "seek --version\nseek --setup\nseek --recent",
  },
  {
    id: "source",
    label: "build from source",
    code: "git clone https://github.com/vishruthb/seek.git\ncd seek\ngo build -o seek .\ninstall -m 0755 ./seek ~/.local/bin/seek",
  },
];

export default function InstallSection() {
  const [activeTab, setActiveTab] = useState("shell");
  const active = tabs.find((tab) => tab.id === activeTab) ?? tabs[0];

  return (
    <section id="install" className="space-y-6 scroll-mt-8">
      <div className="px-1">
        <div className="section-heading">install</div>
        <p className="section-copy mt-3">
          one command to install. one command to configure. then search from
          any repo.
        </p>
      </div>

      <motion.div
        className="section-shell rounded-[1.7rem] p-6 sm:p-8"
        initial={{ opacity: 0, y: 18 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, margin: "-10% 0px" }}
        transition={{ duration: 0.2, ease: "linear" }}
      >
        <div className="mx-auto flex max-w-3xl flex-col items-center gap-6 text-center">
          <h2 className="font-mono text-2xl font-semibold text-text-bright sm:text-3xl">
            install seek in one line
          </h2>
          <p className="max-w-2xl text-sm leading-7 text-text-secondary sm:text-base">
            the installer pulls the latest release from GitHub. run{" "}
            <code className="font-mono text-accent-lime">seek --setup</code>{" "}
            to configure your keys and start searching.
          </p>
          <div className="w-full">
            <CodeBlock code={installCommand} label="curl | sh" pulseOnView />
          </div>
        </div>

        <div className="mt-8 flex flex-wrap gap-3">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              onClick={() => setActiveTab(tab.id)}
              className={`rounded-full border px-4 py-2 font-mono text-sm transition-colors duration-100 ${
                tab.id === activeTab
                  ? "border-border-active bg-accent-glow text-text-bright"
                  : "border-border-subtle text-text-secondary hover:border-border-active hover:text-text-bright"
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        <div className="mt-5">
          <CodeBlock code={active.code} label={active.label} />
        </div>

        <div className="mt-8 grid gap-4 md:grid-cols-3">
          <div className="rounded-2xl border border-border-subtle bg-bg-secondary/90 p-5">
            <div className="font-mono text-sm text-accent-mint">01</div>
            <h3 className="mt-2 font-mono text-lg text-text-bright">
              get tavily
            </h3>
            <p className="mt-2 text-sm leading-7 text-text-secondary">
              grab a free key at{" "}
              <a
                href="https://tavily.com"
                className="text-accent-mint underline decoration-accent-mint-dim underline-offset-4"
              >
                tavily.com
              </a>
              .
            </p>
          </div>
          <div className="rounded-2xl border border-border-subtle bg-bg-secondary/90 p-5">
            <div className="font-mono text-sm text-accent-mint">02</div>
            <h3 className="mt-2 font-mono text-lg text-text-bright">
              pick your backend
            </h3>
            <p className="mt-2 text-sm leading-7 text-text-secondary">
              <a
                href="https://console.groq.com"
                className="text-accent-mint underline decoration-accent-mint-dim underline-offset-4"
              >
                groq
              </a>{" "}
              for speed,{" "}
              <a
                href="https://ollama.com"
                className="text-accent-mint underline decoration-accent-mint-dim underline-offset-4"
              >
                ollama
              </a>{" "}
              to keep answers on your machine, or any OpenAI-compatible API.
            </p>
          </div>
          <div className="rounded-2xl border border-border-subtle bg-bg-secondary/90 p-5">
            <div className="font-mono text-sm text-accent-mint">03</div>
            <h3 className="mt-2 font-mono text-lg text-text-bright">
              run setup
            </h3>
            <p className="mt-2 text-sm leading-7 text-text-secondary">
              run <code className="font-mono text-accent-lime">seek --setup</code>,
              then{" "}
              <code className="font-mono text-accent-lime">seek &quot;review @[main.go]&quot;</code>{" "}
              from any repo.
            </p>
          </div>
        </div>
      </motion.div>
    </section>
  );
}
