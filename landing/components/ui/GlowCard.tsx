import type { ReactNode } from "react";

type GlowCardProps = {
  children: ReactNode;
  className?: string;
};

export default function GlowCard({ children, className = "" }: GlowCardProps) {
  return (
    <div
      className={`section-shell rounded-[1.1rem] p-5 shadow-glow transition-[border-color,box-shadow] duration-150 hover:border-border-active hover:shadow-terminal ${className}`}
    >
      {children}
    </div>
  );
}
