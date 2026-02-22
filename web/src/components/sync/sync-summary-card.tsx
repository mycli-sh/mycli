import { Card } from "../ui/card";
import { Badge } from "../ui/badge";
import { Skeleton } from "../ui/skeleton";
import { useSyncSummary } from "../../hooks/use-sync-summary";

export function SyncSummaryCard() {
  const { data, isLoading } = useSyncSummary();

  if (isLoading) {
    return (
      <Card className="space-y-4">
        <Skeleton className="h-5 w-32" />
        <Skeleton className="h-8 w-20" />
        <Skeleton className="h-4 w-full" />
        <Skeleton className="h-4 w-3/4" />
      </Card>
    );
  }

  if (!data) return null;

  return (
    <Card>
      <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-wider mb-4">
        Sync Summary
      </h2>
      <div className="grid grid-cols-2 gap-4 mb-6">
        <div>
          <div className="text-3xl font-mono font-bold text-zinc-100">
            {data.total_commands}
          </div>
          <div className="text-xs text-zinc-500 mt-1">Total commands</div>
        </div>
        <div>
          <div className="text-3xl font-mono font-bold text-zinc-100">
            {data.user_commands_count}
          </div>
          <div className="text-xs text-zinc-500 mt-1">Your commands</div>
        </div>
      </div>
      {data.installed_libraries.length > 0 && (
        <div>
          <h3 className="text-xs text-zinc-500 uppercase tracking-wider mb-2">
            Installed Libraries
          </h3>
          <div className="space-y-2">
            {data.installed_libraries.map((lib) => (
              <div
                key={lib.slug}
                className="flex items-center justify-between py-1.5"
              >
                <code className="font-mono text-sm text-violet-400">
                  {lib.slug.startsWith("system/") ? lib.slug.slice("system/".length) : lib.slug}
                </code>
                <Badge variant="default">
                  {lib.command_count} commands
                </Badge>
              </div>
            ))}
          </div>
        </div>
      )}
    </Card>
  );
}
