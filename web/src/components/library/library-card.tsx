import { Link } from "react-router";
import { Card } from "../ui/card";
import { Badge } from "../ui/badge";
import type { Library } from "../../lib/api";
import { isSystemOwner } from "../../lib/owner-utils";

interface LibraryCardProps {
  library: Library & { owner: string };
}

export function LibraryCard({ library }: LibraryCardProps) {
  return (
    <Link to={`/libraries/${library.owner}/${library.slug}`}>
      <Card className="hover:border-zinc-700 hover:ring-1 hover:ring-violet-500/50 cursor-pointer group">
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0 flex-1">
            <div className="flex items-center gap-2 mb-1">
              {isSystemOwner(library.owner) ? (
                <Badge variant="official">
                  <svg className="w-3 h-3 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3}>
                    <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
                  </svg>
                  Official
                </Badge>
              ) : (
                <>
                  <span className="text-sm text-zinc-500 font-mono">
                    {library.owner}
                  </span>
                  <span className="text-zinc-700">/</span>
                </>
              )}
              <span className="font-mono font-semibold text-zinc-100 group-hover:text-violet-400 transition-colors">
                {library.slug}
              </span>
            </div>
            <p className="text-sm text-zinc-400 line-clamp-2">
              {library.description || library.name}
            </p>
          </div>
          <Badge variant="success">
            {library.install_count} installs
          </Badge>
        </div>
      </Card>
    </Link>
  );
}
