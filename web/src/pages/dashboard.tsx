import { useAuth } from "../lib/use-auth";
import { SyncSummaryCard } from "../components/sync/sync-summary-card";

export function Dashboard() {
  const { user } = useAuth();

  return (
    <div className="space-y-8">
      <div>
        <h1 className="text-2xl font-bold text-zinc-100 mb-1">
          Welcome back, {user?.username}
        </h1>
        <p className="text-zinc-500 text-sm font-mono">{user?.email}</p>
      </div>

      <SyncSummaryCard />
    </div>
  );
}
