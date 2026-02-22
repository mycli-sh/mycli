import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { sessionApi } from "../lib/api";

export function useSessions() {
  return useQuery({
    queryKey: ["sessions"],
    queryFn: () => sessionApi.list(),
  });
}

export function useRevokeSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => sessionApi.revoke(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["sessions"] }),
  });
}

export function useRevokeAllSessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (currentSessionId: string) => sessionApi.revokeAll(currentSessionId),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["sessions"] }),
  });
}
