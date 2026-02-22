import { Badge } from "../ui/badge";
import { Card } from "../ui/card";
import { InstallSnippet } from "./install-snippet";
import { CommandRow } from "./command-row";
import { ReleaseHistory } from "./release-history";
import type { LibraryDetail as LibraryDetailType } from "../../lib/api";
import { isSystemOwner } from "../../lib/owner-utils";
import { gitUrlToHttp } from "../../lib/git-url";

export function LibraryDetail({ data }: { data: LibraryDetailType }) {
  const { library, owner, commands } = data;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          {isSystemOwner(owner) ? (
            <Badge variant="official">
              <svg className="w-3 h-3 mr-1" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
              </svg>
              Official
            </Badge>
          ) : (
            <>
              <span className="text-zinc-500 font-mono">{owner}</span>
              <span className="text-zinc-700">/</span>
            </>
          )}
          <span className="font-mono font-bold text-2xl text-zinc-100">
            {library.slug}
          </span>
          {library.latest_version && (
            <Badge variant="default">v{library.latest_version}</Badge>
          )}
        </div>
        <p className="text-zinc-400 mb-4">
          {library.description || library.name}
        </p>
        <div className="flex items-center gap-3">
          <Badge variant="success">{library.install_count} installs</Badge>
          {library.is_public && <Badge variant="info">Public</Badge>}
          {library.git_url && (
            <a
              href={gitUrlToHttp(library.git_url)}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-1 text-xs text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6M15 3h6v6M10 14L21 3" />
              </svg>
              Repository
            </a>
          )}
        </div>
      </div>

      {/* Install */}
      <div>
        <h2 className="text-sm font-medium text-zinc-400 mb-2 uppercase tracking-wider">
          Install
        </h2>
        <InstallSnippet owner={owner} slug={library.slug} />
      </div>

      {/* Commands */}
      {commands.length > 0 && (
        <Card>
          <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
            Commands ({commands.length})
          </h2>
          <div className="divide-y divide-zinc-800">
            {commands.map((cmd) => (
              <CommandRow
                key={cmd.command_id}
                command={cmd}
                owner={owner}
                librarySlug={library.slug}
              />
            ))}
          </div>
        </Card>
      )}

      {/* Release History */}
      <ReleaseHistory owner={owner} slug={library.slug} />
    </div>
  );
}
