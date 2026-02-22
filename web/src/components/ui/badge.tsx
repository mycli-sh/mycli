import { cn } from "../../lib/cn";
import type { HTMLAttributes } from "react";

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "success" | "info" | "official";
}

export function Badge({ variant = "default", className, ...props }: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium font-mono",
        variant === "default" && "bg-zinc-800 text-zinc-400",
        variant === "success" && "bg-emerald-500/10 text-emerald-400",
        variant === "info" && "bg-violet-500/10 text-violet-400",
        variant === "official" && "bg-violet-500/10 text-violet-400 border border-violet-500/20",
        className
      )}
      {...props}
    />
  );
}
