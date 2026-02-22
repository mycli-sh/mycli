import { cn } from "../../lib/cn";
import type { InputHTMLAttributes } from "react";

export function Input({
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        "w-full rounded-lg bg-zinc-900 border border-zinc-700 px-4 py-2 text-sm text-zinc-100",
        "placeholder:text-zinc-500 focus:outline-none focus:ring-2 focus:ring-violet-500/50 focus:border-violet-500",
        "transition-all",
        className
      )}
      {...props}
    />
  );
}
