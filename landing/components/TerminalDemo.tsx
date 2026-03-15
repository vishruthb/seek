"use client";

import { FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { motion, useInView } from "framer-motion";

const exampleQuery = "what is a transformer in ML?";

type DemoLine = {
  key: string;
  node: ReactNode;
};

type TerminalDemoProps = {
  compact?: boolean;
};

function DemoText({
  children,
  className = "",
}: {
  children: ReactNode;
  className?: string;
}) {
  return <div className={`leading-7 ${className}`}>{children}</div>;
}

export default function TerminalDemo({ compact = false }: TerminalDemoProps) {
  const terminalRef = useRef<HTMLDivElement | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const isInView = useInView(terminalRef, { once: true, margin: "-15% 0px" });
  const [visibleCount, setVisibleCount] = useState(0);
  const [query, setQuery] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [toastVisible, setToastVisible] = useState(false);
  const [focused, setFocused] = useState(false);

  const lines = useMemo<DemoLine[]>(
    () => [
      {
        key: "cli-header",
        node: (
          <div className="rounded-[0.85rem] border border-border-active/70 bg-accent-glow px-3 py-2 text-[0.78rem] text-text-bright">
            <div className="flex items-center gap-3">
              <span className="font-semibold text-bg-terminal">seek</span>
              <span className="text-text-secondary">│</span>
              <span className="truncate">"{exampleQuery}"</span>
              <span className="ml-auto text-bg-terminal">[1/1]</span>
            </div>
          </div>
        ),
      },
      { key: "gap-head", node: <div className="h-3" /> },
      {
        key: "intro-1",
        node: (
          <DemoText>
            A transformer is a neural network architecture built around{" "}
            <span className="font-semibold text-text-bright">self-attention</span>,
            which lets each token weigh the importance of other tokens in the
            sequence <span className="text-accent-mint">[1][2]</span>.
          </DemoText>
        ),
      },
      {
        key: "intro-2",
        node: (
          <DemoText>
            Instead of processing text strictly left-to-right, it builds a rich
            contextual representation in parallel, which made modern LLMs and
            many vision models practical.
          </DemoText>
        ),
      },
      { key: "gap-1", node: <div className="h-2" /> },
      {
        key: "list-1",
        node: (
          <DemoText>
            <span className="text-accent-mint">1.</span> Tokens attend to one
            another instead of relying on recurrence.
          </DemoText>
        ),
      },
      {
        key: "list-2",
        node: (
          <DemoText>
            <span className="text-accent-mint">2.</span> Multiple attention
            heads capture different relationships at once.
          </DemoText>
        ),
      },
      {
        key: "list-3",
        node: (
          <DemoText>
            <span className="text-accent-mint">3.</span> Positional information
            is added so order is still preserved.
          </DemoText>
        ),
      },
      { key: "gap-3", node: <div className="h-4" /> },
      {
        key: "sources-title",
        node: (
          <div className="border-t border-border-subtle pt-3 text-text-secondary">
            <span className="font-semibold text-text-primary">Sources</span>{" "}
            <span className="text-accent-mint">──────────────────────────────</span>
          </div>
        ),
      },
      {
        key: "source-1",
        node: (
          <div className="rounded-lg bg-bg-tertiary/70 px-2 py-1">
            <span className="mr-2 text-accent-mint">›</span>
            <span className="text-accent-mint">[1]</span> Attention Is All You Need — arxiv.org
            <span className="float-right text-accent-mint">↗</span>
          </div>
        ),
      },
      {
        key: "source-2",
        node: (
          <DemoText className="pl-4 text-text-secondary">
            <span className="text-accent-mint">[2]</span> Transformer overview —
            huggingface.co <span className="float-right text-accent-mint">↗</span>
          </DemoText>
        ),
      },
      {
        key: "source-3",
        node: (
          <DemoText className="pl-4 text-text-secondary">
            <span className="text-accent-mint">[3]</span> Illustrated
            attention — jalammar.github.io{" "}
            <span className="float-right text-accent-mint">↗</span>
          </DemoText>
        ),
      },
      { key: "gap-4", node: <div className="h-4" /> },
      {
        key: "status",
        node: (
          <div className="flex flex-wrap gap-x-4 gap-y-1 rounded-[0.85rem] border border-border-active/60 bg-accent-glow px-3 py-2 text-[0.76rem] text-bg-terminal">
            <span>j/k scroll</span>
            <span>Tab sources</span>
            <span>f follow-up</span>
            <span>Y yank code</span>
            <span className="ml-auto font-semibold">groq/llama-3.3-70b-versatile</span>
          </div>
        ),
      },
    ],
    [],
  );

  useEffect(() => {
    if (!isInView || visibleCount >= lines.length) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      setVisibleCount((current) => {
        if (current >= lines.length) {
          window.clearInterval(timer);
          return current;
        }
        return current + 1;
      });
    }, 70);
    return () => window.clearInterval(timer);
  }, [isInView, lines.length, visibleCount]);

  useEffect(() => {
    if (!toastVisible) {
      return undefined;
    }
    const timer = window.setTimeout(() => setToastVisible(false), 3200);
    return () => window.clearTimeout(timer);
  }, [toastVisible]);

  const interactive = visibleCount >= lines.length;

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    if (!query.trim() || submitting) {
      return;
    }
    setSubmitting(true);
    await new Promise((resolve) => window.setTimeout(resolve, 1500));
    setSubmitting(false);
    setToastVisible(true);
    document.getElementById("install")?.scrollIntoView({ behavior: "smooth", block: "start" });
  }

  const terminal = (
    <motion.div
        ref={terminalRef}
        initial={{ opacity: 0, y: 18 }}
        whileInView={{ opacity: 1, y: 0 }}
        viewport={{ once: true, margin: "-10% 0px" }}
        transition={{ duration: 0.22, ease: "linear" }}
        className={`scanline relative flex h-full flex-col overflow-hidden rounded-[1.6rem] border ${
          focused ? "border-border-active shadow-terminal" : "border-border-subtle shadow-glow"
        } bg-bg-terminal`}
      >
        <div className="flex items-center justify-between border-b border-border-subtle px-4 py-3 text-sm text-text-secondary">
          <div className="flex items-center gap-2">
            <span className="h-3 w-3 rounded-full bg-red-400" />
            <span className="h-3 w-3 rounded-full bg-yellow-400" />
            <span className="h-3 w-3 rounded-full bg-green-400" />
          </div>
          <div className="font-mono uppercase tracking-[0.2em] text-text-primary">
            seek
          </div>
          <div className="w-[3.75rem]" />
        </div>

        <div className="grid flex-1 content-start gap-0 px-4 py-5 font-mono text-[13px] sm:px-6 sm:text-[14px]">
          {lines.slice(0, visibleCount).map((line) => (
            <div key={line.key}>{line.node}</div>
          ))}
          {!interactive && (
            <div className="mt-2 flex items-center gap-2 text-accent-mint">
              <span className="inline-block h-3 w-3 animate-spin rounded-full border border-accent-mint border-t-transparent" />
              <span>Rendering seek demo...</span>
            </div>
          )}
        </div>

        <form
          onSubmit={handleSubmit}
          className={`border-t px-4 py-3 sm:px-6 ${
            focused ? "border-border-active" : "border-border-subtle"
          }`}
        >
          {toastVisible && (
            <div className="mb-3 flex items-center justify-between rounded-xl border border-border-active bg-accent-glow px-4 py-2 text-sm text-text-bright">
              <span>Install seek to search from your real terminal →</span>
              <button
                type="button"
                onClick={() => setToastVisible(false)}
                className="font-mono text-accent-mint"
              >
                dismiss
              </button>
            </div>
          )}
          <div className="flex items-center gap-3 rounded-xl border border-border-subtle bg-bg-secondary/95 px-3 py-3">
            <span className="font-mono text-accent-mint">›</span>
            {submitting ? (
              <div className="flex flex-1 items-center gap-3 font-mono text-sm text-text-secondary">
                <span className="inline-block h-3.5 w-3.5 animate-spin rounded-full border border-accent-mint border-t-transparent" />
                searching...
              </div>
            ) : (
              <>
                <input
                  ref={inputRef}
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  onFocus={() => setFocused(true)}
                  onBlur={() => setFocused(false)}
                  placeholder="Ask anything..."
                  className="w-full bg-transparent font-mono text-sm text-text-bright outline-none placeholder:text-text-secondary"
                />
                <span className="font-mono text-xs text-accent-mint">⏎</span>
              </>
            )}
          </div>
        </form>
      </motion.div>
  );

  if (compact) {
    return terminal;
  }

  return (
    <section className="space-y-5">
      <div className="px-1">
        <div className="section-heading">interactive demo</div>
        <p className="section-copy mt-3">
          this is meant to feel like the real cli: the question sits in the
          header, the answer streams below it, and the follow-up bar stays at
          the bottom.
        </p>
      </div>
      {terminal}
    </section>
  );
}
