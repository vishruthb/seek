"use client";

import { FormEvent, ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { motion, useInView } from "framer-motion";

const exampleQuery = "add chi middleware around @[internal/http/server.go]";
const borderEase = [0.32, 0, 0.18, 1] as const;
const focusTransition = { duration: 0.28, ease: borderEase } as const;

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
              <span className="font-semibold text-text-bright">seek</span>
              <span className="text-text-primary">│</span>
              <span className="truncate">"{exampleQuery}"</span>
              <span className="ml-auto text-text-bright">go/chi</span>
            </div>
          </div>
        ),
      },
      { key: "gap-head", node: <div className="h-3" /> },
      {
        key: "intro-1",
        node: (
          <DemoText>
            The current project already looks like a{" "}
            <span className="font-semibold text-text-bright">Go / Chi</span>{" "}
            service, and the attached file shows where the router is wired up{" "}
            <span className="text-accent-mint">[FILE 1]</span>.
          </DemoText>
        ),
      },
      {
        key: "intro-2",
        node: (
          <DemoText>
            Add the middleware at router construction time so every request
            passes through the same chain before your handlers run{" "}
            <span className="text-accent-mint">[1][2]</span>.
          </DemoText>
        ),
      },
      { key: "gap-1", node: <div className="h-2" /> },
      {
        key: "list-1",
        node: (
          <DemoText>
            <span className="text-accent-mint">1.</span> Create a middleware
            function with the standard{" "}
            <span className="text-text-bright">func(http.Handler) http.Handler</span>{" "}
            shape.
          </DemoText>
        ),
      },
      {
        key: "list-2",
        node: (
          <DemoText>
            <span className="text-accent-mint">2.</span> Register it with{" "}
            <span className="text-text-bright">r.Use(...)</span> close to where
            the router is assembled in the attached file.
          </DemoText>
        ),
      },
      {
        key: "list-3",
        node: (
          <DemoText>
            <span className="text-accent-mint">3.</span> Keep auth, logging,
            and recovery ordered from broadest to most specific so the chain
            stays predictable.
          </DemoText>
        ),
      },
      { key: "gap-3", node: <div className="h-4" /> },
      {
        key: "sources-title",
        node: (
          <div className="border-t border-border-subtle pt-3 text-text-secondary">
            <span className="font-semibold text-text-primary">Sources + file context</span>{" "}
            <span className="text-accent-mint">────────────────────────</span>
          </div>
        ),
      },
      {
        key: "source-1",
        node: (
          <div className="rounded-lg bg-bg-tertiary/70 px-2 py-1">
            <span className="mr-2 text-accent-mint">›</span>
            <span className="text-accent-mint">[1]</span> Chi middleware guide — go-chi.io
            <span className="float-right text-accent-mint">↗</span>
          </div>
        ),
      },
      {
        key: "source-2",
        node: (
          <DemoText className="pl-4 text-text-secondary">
            <span className="text-accent-mint">[2]</span> net/http middleware patterns —
            pkg.go.dev <span className="float-right text-accent-mint">↗</span>
          </DemoText>
        ),
      },
      {
        key: "source-3",
        node: (
          <DemoText className="pl-4 text-text-secondary">
            <span className="text-accent-mint">[FILE 1]</span> internal/http/server.go — attached locally{" "}
            <span className="float-right text-accent-mint">@</span>
          </DemoText>
        ),
      },
      { key: "gap-4", node: <div className="h-4" /> },
      {
        key: "status",
        node: (
          <div className="flex flex-wrap gap-x-4 gap-y-1 rounded-[0.85rem] border border-border-active/50 bg-bg-tertiary/85 px-3 py-2 text-[0.76rem] text-text-bright">
            <span>j/k scroll</span>
            <span>Tab sources</span>
            <span>f follow-up</span>
            <span>@[file] attach</span>
            <span className="ml-auto font-semibold text-accent-mint">642ms · search 211ms · llm 431ms</span>
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
        <div className="pointer-events-none absolute inset-0 rounded-[1.6rem]">
          <motion.div
            className="absolute inset-0 rounded-[1.6rem] border border-accent-mint"
            animate={{
              opacity: focused ? 0.95 : 0.18,
              scale: focused ? 1 : 0.992,
              boxShadow: focused
                ? "0 0 0 1px rgba(166, 227, 161, 0.12) inset, 0 0 28px rgba(166, 227, 161, 0.16)"
                : "0 0 0 1px rgba(166, 227, 161, 0.03) inset, 0 0 0 rgba(166, 227, 161, 0)",
            }}
            transition={focusTransition}
          />
          <motion.div
            className="absolute inset-[1px] rounded-[1.52rem]"
            animate={{
              opacity: focused ? 0.12 : 0,
              background:
                "linear-gradient(180deg, rgba(166, 227, 161, 0.18), rgba(166, 227, 161, 0.04) 34%, rgba(166, 227, 161, 0))",
            }}
            transition={focusTransition}
          />
        </div>
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

        <motion.form
          onSubmit={handleSubmit}
          className="border-t px-4 py-3 sm:px-6"
          animate={{
            borderColor: focused ? "rgba(166, 227, 161, 0.85)" : "rgba(42, 42, 58, 1)",
          }}
          transition={focusTransition}
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
          <motion.div
            className="flex items-center gap-3 rounded-xl border bg-bg-secondary/95 px-3 py-3"
            animate={{
              borderColor: focused ? "rgba(166, 227, 161, 0.9)" : "rgba(42, 42, 58, 1)",
              boxShadow: focused
                ? "0 0 0 1px rgba(166, 227, 161, 0.18), 0 0 22px rgba(166, 227, 161, 0.08)"
                : "0 0 0 0 rgba(166, 227, 161, 0)",
            }}
            transition={focusTransition}
          >
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
                  placeholder='Try: review @[main.go] or /history chi'
                  className="w-full bg-transparent font-mono text-sm text-text-bright outline-none placeholder:text-text-secondary"
                />
                <span className="font-mono text-xs text-accent-mint">⏎</span>
              </>
            )}
          </motion.div>
        </motion.form>
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
          this mirrors the current cli: stack in the header, local file context
          in the prompt, cited sources kept separate, and timing shown once the
          answer lands.
        </p>
      </div>
      {terminal}
    </section>
  );
}
