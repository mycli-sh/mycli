import { useQuery } from "@tanstack/react-query";
import { meApi } from "../lib/api";

export function useSyncSummary() {
  return useQuery({
    queryKey: ["sync-summary"],
    queryFn: () => meApi.syncSummary(),
  });
}
