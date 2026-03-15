"use client";

import { useEffect, useRef, useState } from "react";

import { useInView } from "framer-motion";

type CodeBlockProps = {
  code: string;
  label?: string;
  pulseOnView?: boolean;
  className?: string;
};

export default function CodeBlock({
  code,
  label,
  pulseOnView = false,
  className = "",
}: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);
  const isInView = useInView(ref, { once: true, margin: "-20% 0px" });

  useEffect(() => {
    if (!copied) {
      return undefined;
    }
    const timer = window.setTimeout(() => setCopied(false), 1400);
    return () => window.clearTimeout(timer);
  }, [copied]);

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopied(true);
    } catch {
      setCopied(false);
    }
  }

  return (
    <div
      ref={ref}
      className={`relative rounded-2xl border border-border-subtle bg-bg-terminal/95 shadow-glow ${
        pulseOnView && isInView ? "shadow-terminal" : ""
      } ${className}`}
    >
      <div className="flex items-center justify-between gap-3 border-b border-border-subtle px-4 py-2 text-[0.7rem] uppercase tracking-[0.2em] text-text-secondary">
        <span>{label ?? "Install"}</span>
        <button
          type="button"
          onClick={handleCopy}
          className="inline-flex items-center gap-2 rounded-full border border-border-subtle px-3 py-1 font-mono text-[0.75rem] text-text-primary transition-colors duration-100 hover:border-border-active hover:text-text-bright"
          aria-label="Copy code block"
        >
          {copied ? "✓ Copied" : "Copy"}
        </button>
      </div>
      <button
        type="button"
        onClick={handleCopy}
        className="block w-full overflow-x-auto whitespace-pre px-4 py-4 text-left font-mono text-sm text-accent-lime outline-none"
      >
        <code>{code}</code>
      </button>
    </div>
  );
}
