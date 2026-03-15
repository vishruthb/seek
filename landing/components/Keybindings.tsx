const groups = [
  {
    name: "global",
    rows: [
      ["q", "quit and restore terminal"],
      ["Ctrl+c", "quit immediately"],
      ["Ctrl+l", "redraw screen"],
    ],
  },
  {
    name: "viewing",
    rows: [
      ["j / k", "scroll the answer"],
      ["Tab", "switch to sources"],
      ["f", "open follow-up input"],
      ["/", "search within the answer"],
      ["y / Y", "copy answer or code block"],
    ],
  },
  {
    name: "sources",
    rows: [
      ["j / k", "move selection"],
      ["Enter / o", "open selected source"],
      ["y", "copy selected source URL"],
      ["Tab", "return to summary"],
    ],
  },
];

export default function Keybindings() {
  return (
    <section className="space-y-6">
      <div className="px-1">
        <div className="section-heading">keybindings</div>
        <p className="section-copy mt-3">
          the interface is keyboard-first. if you already live in `less`, `vim`,
          `fzf`, or `lazygit`, the interaction model should feel familiar.
        </p>
      </div>

      <div className="section-shell overflow-hidden rounded-[1.4rem]">
        <div className="border-b border-border-subtle px-5 py-3 font-mono text-xs uppercase tracking-[0.2em] text-text-secondary">
          seek --help
        </div>
        <div className="grid gap-0 md:grid-cols-3">
          {groups.map((group, index) => (
            <div
              key={group.name}
              className={`${index < groups.length - 1 ? "md:border-r" : ""} border-border-subtle p-5`}
            >
              <div className="mb-4 font-mono text-sm text-accent-mint">
                {group.name}
              </div>
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
      </div>
    </section>
  );
}
