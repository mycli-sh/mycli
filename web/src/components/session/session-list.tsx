import { Card } from "../ui/card";
import { Button } from "../ui/button";
import { Skeleton } from "../ui/skeleton";
import { SessionRow } from "./session-row";
import { useSessions, useRevokeSession, useRevokeAllSessions } from "../../hooks/use-sessions";
import { useAuth } from "../../lib/use-auth";

export function SessionList() {
  const { sessionId } = useAuth();
  const { data, isLoading } = useSessions();
  const revoke = useRevokeSession();
  const revokeAll = useRevokeAllSessions();

  if (isLoading) {
    return (
      <Card className="space-y-4">
        <Skeleton className="h-5 w-40" />
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-16 w-full" />
      </Card>
    );
  }

  const sessions = data?.sessions ?? [];
  const otherSessions = sessions.filter((s) => s.id !== sessionId);

  return (
    <Card>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-medium text-zinc-400 uppercase tracking-wider">
          Active Sessions ({sessions.length})
        </h2>
        {otherSessions.length > 0 && sessionId && (
          <Button
            variant="danger"
            size="sm"
            onClick={() => revokeAll.mutate(sessionId)}
            disabled={revokeAll.isPending}
          >
            Revoke all others
          </Button>
        )}
      </div>
      {sessions.length === 0 ? (
        <p className="text-sm text-zinc-500">No active sessions.</p>
      ) : (
        <div>
          {sessions.map((s) => (
            <SessionRow
              key={s.id}
              session={s}
              isCurrent={s.id === sessionId}
              onRevoke={(id) => revoke.mutate(id)}
              isRevoking={revoke.isPending}
            />
          ))}
        </div>
      )}
    </Card>
  );
}
