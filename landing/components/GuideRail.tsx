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
    text: "keep inference on your own machine with ollama, or switch to groq, openrouter, and together for cloud speed.",
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
    body: "your selected model reads the search context and answers with citations, whether that's ollama locally or a fast hosted backend.",
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
        rootMargin: "-18% 0px -48% 0px",
        threshold: [0.25, 0.45, 0.65],
      },
    );

    entries.forEach((entry) => observer.observe(entry));
    return () => observer.disconnect();
  }, []);

  function scrollToSection(id: (typeof sections)[number]["id"]) {
    observerRefs.current[id].current?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  }

  return (
    <div className="grid w-full min-w-0 gap-10 lg:grid-cols-[240px_minmax(0,1fr)] lg:gap-12 xl:grid-cols-[270px_minmax(0,1fr)] xl:gap-14">
      <div className="hidden lg:block lg:sticky lg:top-24 lg:self-start">
        <div className="section-heading">inside seek</div>
        <div className="mt-6 flex flex-col gap-3">
          {sections.map((section) => {
            const isActive = section.id === active;
            return (
              <button
                key={section.id}
                type="button"
                onClick={() => scrollToSection(section.id)}
                className={`rounded-[1.2rem] border px-4 py-4 text-left transition-all duration-200 ${
                  isActive
                    ? "border-border-active bg-accent-glow text-text-bright shadow-glow"
                    : "border-border-subtle text-text-secondary/75"
                }`}
              >
                <div className={`font-mono transition-all duration-150 ${isActive ? "text-xl text-accent-mint" : "text-sm"}`}>
                  {section.title}
                </div>
                <div className={`mt-2 max-w-[18rem] text-sm leading-6 transition-all duration-150 ${
                  isActive ? "text-text-primary" : "text-text-secondary/70"
                }`}>
                  {section.summary}
                </div>
              </button>
            );
          })}
        </div>
      </div>

      <div className="min-w-0">
        <div className="max-w-2xl px-1 lg:hidden">
          <div className="section-heading">inside seek</div>
          <p className="section-copy mt-3">
            one flow, three parts: what seek gives you, how the search stack works,
            and the keys that keep it fast once your hands are already on the keyboard.
          </p>
        </div>

        <div className="sticky top-3 z-20 mt-6 -mx-1 flex gap-3 overflow-x-auto px-1 pb-3 lg:hidden">
          {sections.map((section) => {
            const isActive = section.id === active;
            return (
              <button
                key={section.id}
                type="button"
                onClick={() => scrollToSection(section.id)}
                className={`shrink-0 rounded-full border px-4 py-2 font-mono text-sm transition-all duration-200 ${
                  isActive
                    ? "border-border-active bg-accent-glow text-text-bright shadow-glow"
                    : "border-border-subtle bg-bg-secondary/90 text-text-secondary"
                }`}
              >
                {section.title}
              </button>
            );
          })}
        </div>

        <div className="space-y-14 pb-8 sm:space-y-16 lg:space-y-[4.5rem] lg:pb-36 xl:space-y-20 xl:pb-44">
          <section
            id="features"
            ref={(node) => {
              observerRefs.current.features.current = node;
            }}
            className={`section-shell scroll-mt-24 rounded-[1.8rem] p-6 transition-all duration-200 sm:p-8 ${
              active === "features" ? "opacity-100 blur-0" : "lg:opacity-45 lg:blur-[1.5px]"
            }`}
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
                  className="min-w-0 rounded-[1.2rem] border border-border-subtle bg-bg-secondary/90 p-5"
                >
                  <div className="flex items-start gap-4">
                    <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-lg border border-border-subtle bg-bg-tertiary font-mono text-sm font-semibold text-accent-mint">
                      {item.icon}
                    </div>
                    <div className="min-w-0 space-y-2">
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
            className={`section-shell scroll-mt-24 rounded-[1.8rem] p-6 transition-all duration-200 sm:p-8 ${
              active === "how-it-works" ? "opacity-100 blur-0" : "lg:opacity-45 lg:blur-[1.5px]"
            }`}
          >
            <div className="max-w-2xl">
              <div className="section-heading">how it works</div>
              <p className="section-copy mt-3">
                seek separates search from synthesis. tavily fetches the web results,
                then your llm reads them and writes the answer. pair tavily with
                ollama for a local pass, or swap in groq when you want faster hosted
                responses.
              </p>
            </div>
            <div className="mt-8 flex flex-col gap-5 xl:flex-row xl:items-stretch">
              {howSteps.map((step, index) => (
                <div key={step.label} className="flex flex-1 items-stretch gap-5">
                  <div className="min-w-0 flex-1 rounded-[1.2rem] border border-border-subtle bg-bg-secondary/90 p-5">
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
            className={`section-shell scroll-mt-28 overflow-hidden rounded-[1.8rem] transition-all duration-200 ${
              active === "keybindings" ? "opacity-100 blur-0" : "lg:opacity-45 lg:blur-[1.5px]"
            }`}
          >
            <div className="border-b border-border-subtle px-6 py-4 font-mono text-xs tracking-[0.2em] text-text-secondary">
              keybindings
            </div>
            <div className="grid gap-0 lg:grid-cols-3">
              {keyGroups.map((group, index) => (
                <div
                  key={group.name}
                  className={`${index < keyGroups.length - 1 ? "lg:border-r" : ""} min-w-0 border-border-subtle p-6`}
                >
                  <div className="mb-4 font-mono text-sm text-accent-mint">{group.name}</div>
                  <div className="space-y-3 font-mono text-sm">
                    {group.rows.map(([key, action]) => (
                      <div key={key} className="flex items-start gap-3">
                        <span className="shrink-0 rounded-md border border-border-subtle bg-bg-tertiary px-2 py-1 text-accent-lime">
                          {key}
                        </span>
                        <span className="min-w-0 leading-6 text-text-secondary">{action}</span>
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </section>

          <div aria-hidden="true" className="hidden lg:block lg:h-16 xl:h-20" />
        </div>
      </div>
    </div>
  );
}
