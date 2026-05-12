import type { HTMLAttributes } from "react";

export function Card({
  className = "",
  ...rest
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      {...rest}
      className={[
        "rounded-lg border border-ink-200 bg-white/80 backdrop-blur-sm",
        "shadow-[0_1px_0_rgba(0,0,0,0.02)]",
        className,
      ].join(" ")}
    />
  );
}
