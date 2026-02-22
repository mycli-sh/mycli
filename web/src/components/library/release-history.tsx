import { Badge } from "../ui/badge";
import { Card } from "../ui/card";
import { useLibraryReleases } from "../../hooks/use-libraries";

interface ReleaseHistoryProps {
  owner: string;
  slug: string;
}

export function ReleaseHistory({ owner, slug }: ReleaseHistoryProps) {
  const { data, isLoading } = useLibraryReleases(owner, slug);
  const releases = data?.releases ?? [];

  if (isLoading) {
    return (
      <Card>
        <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
          Releases
        </h2>
        <div className="animate-pulse space-y-3">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-8 bg-zinc-800 rounded" />
          ))}
        </div>
      </Card>
    );
  }

  if (releases.length === 0) return null;

  return (
    <Card>
      <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
        Releases ({releases.length})
      </h2>
      <div className="divide-y divide-zinc-800">
        {releases.map((release, i) => (
          <div key={release.id} className="flex items-center gap-3 py-3 first:pt-0 last:pb-0">
            <Badge variant={i === 0 ? "info" : "default"}>
              {release.tag}
            </Badge>
            <span className="text-sm text-zinc-300">
              {release.command_count} command{release.command_count !== 1 ? "s" : ""}
            </span>
            {release.commit_hash && (
              <code className="text-xs text-zinc-600 font-mono">
                {release.commit_hash.slice(0, 7)}
              </code>
            )}
            <span className="text-xs text-zinc-600 ml-auto">
              {new Date(release.released_at).toLocaleDateString()}
            </span>
          </div>
        ))}
      </div>
    </Card>
  );
}
