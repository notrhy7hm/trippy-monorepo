import type { InputHTMLAttributes } from "react";

export function Input({
  className = "",
  ...rest
}: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      {...rest}
      className={[
        "w-full rounded-md border border-ink-200 bg-white px-3 py-2 text-sm",
        "placeholder:text-ink-400 focus:border-ink-500 focus:outline-none",
        "focus:ring-1 focus:ring-ink-500",
        className,
      ].join(" ")}
    />
  );
}
