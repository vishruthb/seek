"use client";

import { useEffect, useState } from "react";

type TypingEffectProps = {
  text: string;
  speed?: number;
  className?: string;
};

export default function TypingEffect({
  text,
  speed = 40,
  className,
}: TypingEffectProps) {
  const [visible, setVisible] = useState("");

  useEffect(() => {
    let index = 0;
    const timer = window.setInterval(() => {
      index += 1;
      setVisible(text.slice(0, index));
      if (index >= text.length) {
        window.clearInterval(timer);
      }
    }, speed);
    return () => window.clearInterval(timer);
  }, [speed, text]);

  return (
    <span className={className}>
      {visible}
      <span className="ml-1 inline-block h-[1em] w-[0.62ch] animate-blink bg-accent-mint align-[-0.1em]" />
    </span>
  );
}
