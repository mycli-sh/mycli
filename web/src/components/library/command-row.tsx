import { Link } from "react-router";
import type { LibraryCommand } from "../../lib/api";

interface CommandRowProps {
  command: LibraryCommand;
  owner: string;
  librarySlug: string;
}

export function CommandRow({ command, owner, librarySlug }: CommandRowProps) {
  return (
    <Link
      to={`/libraries/${owner}/${librarySlug}/commands/${command.slug}`}
      className="flex items-start gap-4 py-3 border-b border-zinc-800 last:border-0 group hover:bg-zinc-800/50 -mx-2 px-2 rounded-md transition-colors"
    >
      <code className="font-mono text-sm text-violet-400 shrink-0 min-w-[120px]">
        {command.slug}
      </code>
      <div className="min-w-0 flex-1">
        <div className="text-sm text-zinc-100">{command.name}</div>
        {command.description && (
          <div className="text-xs text-zinc-500 mt-0.5">{command.description}</div>
        )}
      </div>
      <svg
        className="w-4 h-4 text-zinc-600 group-hover:text-zinc-400 mt-1 shrink-0 transition-colors"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
        strokeWidth={2}
      >
        <path strokeLinecap="round" strokeLinejoin="round" d="M9 5l7 7-7 7" />
      </svg>
    </Link>
  );
}
