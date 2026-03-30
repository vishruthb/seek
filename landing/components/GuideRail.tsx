"use client";

import { useEffect, useMemo, useRef, useState } from "react";

const sections = [
  {
    id: "features",
    title: "features",
    summary: "project context, file attachments, local history, keyboard-first navigation.",
  },
  {
    id: "how-it-works",
    title: "how it works",
    summary: "repo context shapes the query. web results ground the answer.",
  },
  {
    id: "keybindings",
    title: "keybindings",
    summary: "vim-style navigation, slash commands, and inline follow-ups.",
  },
] as const;

const featureItems = [
  {
    icon: "ctx",
    title: "project-aware by default",
    text: "seek reads your repo to detect the language, framework, and dependencies, then biases every search toward your stack.",
  },
  {
    icon: "@[ ]",
    title: "attach local files inline",
    text: "type @[file] to attach code from your project. seek autocompletes paths and sends file contents alongside search results to your LLM.",
  },
  {
    icon: "db",
    title: "persistent local history",
    text: "every search is saved locally with full context. search your history, reopen past sessions, and pick up where you left off.",
  },
  {
    icon: "ms",
    title: "visible latency",
    text: "the status bar breaks down response time into search and LLM components so you always know where time went.",
  },
  {
    icon: "jk",
    title: "keyboard-first controls",
    text: "j/k navigation, slash command autocomplete, and file path suggestions. everything responds to the keyboard.",
  },
  {
    icon: "llm",
    title: "local or hosted backends",
    text: "run answers locally with Ollama or route through Groq, OpenRouter, or any OpenAI-compatible API. switch mid-session with /backend.",
  },
];

const howSteps = [
  {
    label: "01",
    title: 'seek "explain @[main.go]"',
    body: "ask from the repo you are already in. seek pulls project context and local file contents into the same request.",
  },
  {
    label: "02",
    title: "repo context + tavily search",
    body: "seek biases the query toward your stack, fetches fresh pages, and keeps numbered sources separate from the answer.",
  },
  {
    label: "03",
    title: "your llm streams the answer",
    body: "your chosen backend reads the search results and attached files, streams a cited answer, and seek saves the session locally.",
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
      ["@[file]", "attach local file context"],
      ["/history", "search local history"],
      ["/context", "inspect or toggle detected stack"],
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
            what seek adds, how the pipeline works, and the keybindings that
            keep it fast.
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
                everything you need to stay in the terminal and stop
                context-switching to a browser.
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
                seek separates search from synthesis. your repo context shapes
                the query, Tavily fetches live results, and your LLM answers
                against both.
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
            <div className="border-b border-border-subtle px-6 py-5">
              <div className="section-heading">keybindings</div>
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
