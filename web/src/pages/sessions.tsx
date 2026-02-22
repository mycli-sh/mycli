import { SessionList } from "../components/session/session-list";

export function Sessions() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-zinc-100 mb-1">Sessions</h1>
        <p className="text-sm text-zinc-500">
          Manage your active sessions across devices.
        </p>
      </div>
      <SessionList />
    </div>
  );
}
