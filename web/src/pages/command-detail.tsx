import { useParams, Link } from "react-router";
import { Skeleton } from "../components/ui/skeleton";
import { CommandDetailView } from "../components/library/command-detail-view";
import { useLibraryCommand, useCommandVersions } from "../hooks/use-libraries";
import { isSystemOwner } from "../lib/owner-utils";

export function CommandDetailPage() {
  const { owner, slug, commandSlug } = useParams<{
    owner: string;
    slug: string;
    commandSlug: string;
  }>();

  const { data: commandData, isLoading: cmdLoading, error: cmdError } =
    useLibraryCommand(owner!, slug!, commandSlug!);
  const { data: versionsData, isLoading: versionsLoading } =
    useCommandVersions(owner!, slug!, commandSlug!);

  const isLoading = cmdLoading || versionsLoading;

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-4 w-32" />
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96" />
        <Skeleton className="h-64 w-full rounded-xl" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  if (cmdError || !commandData) {
    return (
      <div className="text-center py-12">
        <p className="text-zinc-500 mb-4">Command not found.</p>
        <Link
          to={`/libraries/${owner}/${slug}`}
          className="text-violet-400 hover:text-violet-300 text-sm"
        >
          Back to library
        </Link>
      </div>
    );
  }

  return (
    <div>
      <Link
        to={`/libraries/${owner}/${slug}`}
        className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 mb-6 transition-colors"
      >
        <svg
          className="w-4 h-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        {isSystemOwner(owner) && (
          <svg className="w-3.5 h-3.5 text-violet-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={3}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M5 13l4 4L19 7" />
          </svg>
        )}
        {isSystemOwner(owner) ? slug : `${owner}/${slug}`}
      </Link>
      <CommandDetailView
        data={commandData}
        versions={versionsData?.versions ?? []}
        librarySlug={slug}
      />
    </div>
  );
}
