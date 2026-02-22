import { cn } from "../../lib/cn";
import type { HTMLAttributes } from "react";

export function Card({ className, ...props }: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-xl bg-zinc-900 border border-zinc-800 p-6",
        "transition-all",
        className
      )}
      {...props}
    />
  );
}
