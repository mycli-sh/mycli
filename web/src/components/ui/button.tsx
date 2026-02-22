import { cn } from "../../lib/cn";
import type { ButtonHTMLAttributes } from "react";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "danger" | "ghost";
  size?: "sm" | "md";
}

export function Button({
  variant = "primary",
  size = "md",
  className,
  ...props
}: ButtonProps) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center rounded-lg font-medium transition-all cursor-pointer",
        "disabled:opacity-50 disabled:cursor-not-allowed",
        size === "sm" && "px-3 py-1.5 text-sm",
        size === "md" && "px-4 py-2 text-sm",
        variant === "primary" &&
          "bg-gradient-to-r from-violet-500 to-violet-600 text-white hover:from-violet-600 hover:to-violet-700",
        variant === "secondary" &&
          "bg-zinc-800 text-zinc-100 hover:bg-zinc-700 border border-zinc-700",
        variant === "danger" &&
          "bg-red-500/10 text-red-400 hover:bg-red-500/20 border border-red-500/20",
        variant === "ghost" &&
          "text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800",
        className
      )}
      {...props}
    />
  );
}
