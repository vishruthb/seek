"use client";

import { useEffect, useMemo, useRef, useState } from "react";

const sections = [
  {
    id: "features",
    title: "features",
    summary: "fast search, local or cloud backends, and keyboard-first reading.",
  },
  {
    id: "how-it-works",
    title: "how it works",
    summary: "search first, synthesis second. tavily gathers context, your llm answers.",
  },
  {
    id: "keybindings",
    title: "keybindings",
    summary: "built for people who already live in vim, less, fzf, and lazygit.",
  },
] as const;

const featureItems = [
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
    text: "press f to ask follow-ups. context stays in the same session.",
  },
  {
    icon: "llm",
    title: "pluggable backends",
    text: "ollama for local answer generation. groq, openrouter, and together for cloud.",
  },
  {
    icon: "Y",
    title: "yank code blocks",
    text: "press Y to copy a fenced code block. multiple blocks can be selected by number.",
  },
  {
    icon: "/",
    title: "configurable modes",
    text: "switch between concise, learning, explanatory, and oneliner with /mode.",
  },
];

const howSteps = [
  {
    label: "01",
    title: 'seek "your question"',
    body: "you stay in the terminal and ask the question the moment it appears.",
  },
  {
    label: "02",
    title: "tavily searches the web",
    body: "fresh pages, extracted content, and citations are gathered quickly.",
  },
  {
    label: "03",
    title: "your llm synthesizes",
    body: "ollama or an openai-compatible backend reads the results and answers with citations.",
  },
];

const keyGroups = [
  {
    name: "global",
    rows: [
      ["q", "quit and restore terminal"],
      ["ctrl+c", "quit immediately"],
      ["ctrl+l", "redraw screen"],
    ],
  },
  {
    name: "viewing",
    rows: [
      ["j / k", "scroll the answer"],
      ["tab", "switch to sources"],
      ["f", "open follow-up input"],
      ["/", "search within the answer"],
      ["y / Y", "copy answer or code block"],
    ],
  },
  {
    name: "sources",
    rows: [
      ["j / k", "move selection"],
      ["enter / o", "open selected source"],
      ["y", "copy selected source url"],
      ["tab", "return to summary"],
    ],
  },
];

export default function GuideRail() {
  const [active, setActive] = useState<(typeof sections)[number]["id"]>("features");
  const refs = useMemo(
    () =>
      Object.fromEntries(
        sections.map((section) => [section.id, { current: null as HTMLElement | null }]),
      ) as Record<(typeof sections)[number]["id"], { current: HTMLElement | null }>,
    [],
  );
  const observerRefs = useRef(refs);

  useEffect(() => {
    const entries = Object.values(observerRefs.current)
      .map((ref) => ref.current)
      .filter((value): value is HTMLElement => value !== null);
    if (entries.length === 0) {
      return undefined;
    }

    const observer = new IntersectionObserver(
      (observed) => {
        const visible = observed
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => b.intersectionRatio - a.intersectionRatio);
        if (visible.length === 0) {
          return;
        }
        setActive(visible[0].target.id as (typeof sections)[number]["id"]);
      },
      {
        rootMargin: "-22% 0px -38% 0px",
        threshold: [0.25, 0.45, 0.65],
      },
    );

    entries.forEach((entry) => observer.observe(entry));
    return () => observer.disconnect();
  }, []);

  return (
    <div className="grid w-full gap-8 lg:grid-cols-[220px_minmax(0,1fr)] xl:grid-cols-[260px_minmax(0,1fr)] xl:gap-12">
      <div className="lg:sticky lg:top-20 lg:self-start">
        <div className="section-heading">inside seek</div>
        <div className="mt-5 flex gap-2 overflow-x-auto pb-1 lg:flex-col lg:overflow-visible">
          {sections.map((section) => {
            const isActive = section.id == active;
            return (
              <button
                key={section.id}
                type="button"
                onClick={() =>
                  observerRefs.current[section.id].current?.scrollIntoView({
                    behavior: "smooth",
                    block: "center",
                  })
                }
                className={`rounded-[1.1rem] border px-4 py-3 text-left transition-all duration-150 ${
                  isActive
                    ? "border-border-active bg-accent-glow text-text-bright shadow-glow"
                    : "border-border-subtle text-text-secondary"
                }`}
              >
                <div
                  className={`font-mono transition-all duration-150 ${
                    isActive ? "text-xl text-accent-mint" : "text-sm"
                  }`}
                >
                  {section.title}
                </div>
                <div
                  className={`mt-2 max-w-[18rem] text-sm leading-6 transition-all duration-150 ${
                    isActive ? "block text-text-primary" : "hidden lg:block lg:text-text-secondary/70"
                  }`}
                >
                  {section.summary}
                </div>
              </button>
            );
          })}
        </div>
      </div>

      <div className="space-y-10">
        <section
          id="features"
          ref={(node) => {
            observerRefs.current.features.current = node;
          }}
          className="section-shell rounded-[1.8rem] p-6 sm:p-8"
        >
          <div className="max-w-2xl">
            <div className="section-heading">features</div>
            <p className="section-copy mt-3">
              it's built for the moment you need an answer now, not after a browser
              detour. one binary, one terminal, one flow.
            </p>
          </div>
          <div className="mt-8 grid gap-4 md:grid-cols-2">
            {featureItems.map((item) => (
              <div
                key={item.title}
                className="rounded-[1.2rem] border border-border-subtle bg-bg-secondary/90 p-5"
              >
                <div className="flex items-start gap-4">
                  <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-border-subtle bg-bg-tertiary font-mono text-sm font-semibold text-accent-mint">
                    {item.icon}
                  </div>
                  <div className="space-y-2">
                    <h3 className="font-mono text-lg font-semibold text-text-bright">
                      {item.title}
                    </h3>
                    <p className="text-sm leading-7 text-text-secondary">{item.text}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section
          id="how-it-works"
          ref={(node) => {
            observerRefs.current["how-it-works"].current = node;
          }}
          className="section-shell rounded-[1.8rem] p-6 sm:p-8"
        >
          <div className="max-w-2xl">
            <div className="section-heading">how it works</div>
            <p className="section-copy mt-3">
              seek separates search from synthesis. tavily fetches the web results,
              then your llm reads them and writes the answer. use ollama when you
              want local answer generation, or use groq when you want speed.
            </p>
          </div>
          <div className="mt-8 flex flex-col gap-5 xl:flex-row xl:items-stretch">
            {howSteps.map((step, index) => (
              <div key={step.label} className="flex flex-1 items-stretch gap-5">
                <div className="flex-1 rounded-[1.2rem] border border-border-subtle bg-bg-secondary/90 p-5">
                  <div className="mb-4 font-mono text-sm text-accent-mint">{step.label}</div>
                  <h3 className="font-mono text-lg font-semibold text-text-bright">
                    {step.title}
                  </h3>
                  <p className="mt-3 text-sm leading-7 text-text-secondary">{step.body}</p>
                </div>
                {index < howSteps.length - 1 && (
                  <div className="hidden items-center xl:flex">
                    <div className="h-px w-16 border-t border-dashed border-accent-mint-dim" />
                  </div>
                )}
              </div>
            ))}
          </div>
        </section>

        <section
          id="keybindings"
          ref={(node) => {
            observerRefs.current.keybindings.current = node;
          }}
          className="section-shell overflow-hidden rounded-[1.8rem]"
        >
          <div className="border-b border-border-subtle px-6 py-4 font-mono text-xs tracking-[0.2em] text-text-secondary">
            keybindings
          </div>
          <div className="grid gap-0 lg:grid-cols-3">
            {keyGroups.map((group, index) => (
              <div
                key={group.name}
                className={`${index < keyGroups.length - 1 ? "lg:border-r" : ""} border-border-subtle p-6`}
              >
                <div className="mb-4 font-mono text-sm text-accent-mint">{group.name}</div>
                <div className="space-y-3 font-mono text-sm">
                  {group.rows.map(([key, action]) => (
                    <div key={key} className="flex items-start gap-3">
                      <span className="rounded-md border border-border-subtle bg-bg-tertiary px-2 py-1 text-accent-lime">
                        {key}
                      </span>
                      <span className="leading-6 text-text-secondary">{action}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
