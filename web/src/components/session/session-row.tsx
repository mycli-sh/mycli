import { Button } from "../ui/button";
import { Badge } from "../ui/badge";
import type { Session } from "../../lib/api";

interface SessionRowProps {
  session: Session;
  isCurrent: boolean;
  onRevoke: (id: string) => void;
  isRevoking: boolean;
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function parseUserAgent(ua: string) {
  if (!ua) return "Unknown device";
  if (ua.startsWith("mycli/")) return `mycli CLI (${ua})`;
  if (ua.includes("mycli")) return "mycli CLI";
  if (ua.includes("Chrome")) return "Chrome";
  if (ua.includes("Firefox")) return "Firefox";
  if (ua.includes("Safari")) return "Safari";
  return ua.slice(0, 40);
}

function formatDeviceName(name: string) {
  // Strip common suffixes like ".local" for cleaner display
  return name.replace(/\.local$/, "");
}

export function SessionRow({
  session,
  isCurrent,
  onRevoke,
  isRevoking,
}: SessionRowProps) {
  return (
    <div className="flex items-center gap-4 py-4 border-b border-zinc-800 last:border-0">
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2 mb-1">
          <span className="text-sm font-medium text-zinc-100">
            {session.device_name
              ? formatDeviceName(session.device_name)
              : parseUserAgent(session.user_agent)}
          </span>
          {isCurrent && <Badge variant="info">Current</Badge>}
        </div>
        <div className="flex items-center gap-3 text-xs text-zinc-500">
          <span>{parseUserAgent(session.user_agent)}</span>
          <span>{session.ip_address}</span>
          <span>Last used {formatDate(session.last_used_at)}</span>
          <span>Created {formatDate(session.created_at)}</span>
        </div>
      </div>
      {!isCurrent && (
        <Button
          variant="danger"
          size="sm"
          onClick={() => onRevoke(session.id)}
          disabled={isRevoking}
        >
          Revoke
        </Button>
      )}
    </div>
  );
}
