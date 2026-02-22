import { useState, useCallback } from "react";
import { SearchInput } from "../components/ui/search-input";
import { Skeleton } from "../components/ui/skeleton";
import { LibraryCard } from "../components/library/library-card";
import { useLibrarySearch } from "../hooks/use-libraries";

export function Home() {
  const [query, setQuery] = useState("");
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const { data, isLoading, isFetching, error, refetch } = useLibrarySearch(query, limit, offset);

  const handleSearch = useCallback((value: string) => {
    setQuery(value);
    setOffset(0);
  }, []);

  const total = data?.total ?? 0;
  const libraries = data?.libraries ?? [];
  const hasMore = offset + limit < total;
  const hasPrev = offset > 0;

  return (
    <div className="space-y-8">
      {/* Hero */}
      <div className="text-center py-8">
        <h1 className="text-3xl sm:text-4xl font-bold text-zinc-100 mb-3">
          Explore <span className="text-violet-400">Libraries</span>
        </h1>
        <p className="text-zinc-400 text-lg mb-8 max-w-lg mx-auto">
          Browse public command libraries. Install with a single command.
        </p>
        <SearchInput
          value={query}
          onChange={handleSearch}
          placeholder="Search libraries..."
          className="max-w-xl mx-auto"
        />
      </div>

      {/* Results */}
      {error ? (
        <div className="text-center py-12">
          <p className="text-red-400 mb-4">Failed to load libraries.</p>
          <button
            onClick={() => refetch()}
            className="text-sm text-violet-400 hover:text-violet-300 transition-colors cursor-pointer"
          >
            Try again
          </button>
        </div>
      ) : isLoading ? (
        <div className="grid gap-4 sm:grid-cols-2">
          {Array.from({ length: 4 }).map((_, i) => (
            <Skeleton key={i} className="h-28 rounded-xl" />
          ))}
        </div>
      ) : libraries.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-zinc-500">
            {query ? "No libraries found matching your search." : "No libraries available yet."}
          </p>
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between">
            <p className="text-sm text-zinc-500">
              {total} {total === 1 ? "library" : "libraries"} found
              {isFetching && " ..."}
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-2">
            {libraries.map((lib) => (
              <LibraryCard key={lib.id} library={lib} />
            ))}
          </div>
          {(hasMore || hasPrev) && (
            <div className="flex items-center justify-center gap-4 pt-4">
              <button
                onClick={() => setOffset(Math.max(0, offset - limit))}
                disabled={!hasPrev}
                className="text-sm text-zinc-400 hover:text-zinc-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors cursor-pointer"
              >
                Previous
              </button>
              <span className="text-sm text-zinc-600">
                Page {Math.floor(offset / limit) + 1}
              </span>
              <button
                onClick={() => setOffset(offset + limit)}
                disabled={!hasMore}
                className="text-sm text-zinc-400 hover:text-zinc-100 disabled:opacity-30 disabled:cursor-not-allowed transition-colors cursor-pointer"
              >
                Next
              </button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
