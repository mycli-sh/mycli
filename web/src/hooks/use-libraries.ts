import { useQuery, keepPreviousData } from "@tanstack/react-query";
import { libraryApi } from "../lib/api";

export function useLibrarySearch(query: string, limit = 20, offset = 0) {
  return useQuery({
    queryKey: ["libraries", "search", query, limit, offset],
    queryFn: () => libraryApi.search(query, limit, offset),
    placeholderData: keepPreviousData,
  });
}

export function useLibraryDetail(owner: string, slug: string) {
  return useQuery({
    queryKey: ["libraries", "detail", owner, slug],
    queryFn: () => libraryApi.getDetail(owner, slug),
    enabled: !!owner && !!slug,
  });
}

export function useLibraryReleases(owner: string, slug: string) {
  return useQuery({
    queryKey: ["libraries", "releases", owner, slug],
    queryFn: () => libraryApi.listReleases(owner, slug),
    enabled: !!owner && !!slug,
  });
}

export function useLibraryCommand(owner: string, slug: string, commandSlug: string) {
  return useQuery({
    queryKey: ["libraries", "command", owner, slug, commandSlug],
    queryFn: () => libraryApi.getCommand(owner, slug, commandSlug),
    enabled: !!owner && !!slug && !!commandSlug,
  });
}

export function useCommandVersions(owner: string, slug: string, commandSlug: string) {
  return useQuery({
    queryKey: ["libraries", "command-versions", owner, slug, commandSlug],
    queryFn: () => libraryApi.listCommandVersions(owner, slug, commandSlug),
    enabled: !!owner && !!slug && !!commandSlug,
  });
}
