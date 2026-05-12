import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost";

const styles: Record<Variant, string> = {
  primary:
    "bg-ink-950 text-white hover:bg-ink-800 disabled:opacity-50",
  secondary:
    "bg-white text-ink-900 border border-ink-200 hover:border-ink-400 disabled:opacity-50",
  ghost:
    "bg-transparent text-ink-700 hover:text-ink-950 disabled:opacity-50",
};

export function Button({
  variant = "primary",
  className = "",
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      {...rest}
      className={[
        "inline-flex items-center justify-center rounded-md px-4 py-2 text-sm font-medium",
        "transition-colors focus:outline-none focus:ring-2 focus:ring-ink-300",
        styles[variant],
        className,
      ].join(" ")}
    />
  );
}
