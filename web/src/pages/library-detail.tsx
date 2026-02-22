import { useParams, Link } from "react-router";
import { Skeleton } from "../components/ui/skeleton";
import { LibraryDetail } from "../components/library/library-detail";
import { useLibraryDetail } from "../hooks/use-libraries";

export function LibraryDetailPage() {
  const { owner, slug } = useParams<{ owner: string; slug: string }>();
  const { data, isLoading, error } = useLibraryDetail(owner!, slug!);

  if (isLoading) {
    return (
      <div className="space-y-6">
        <Skeleton className="h-8 w-64" />
        <Skeleton className="h-4 w-96" />
        <Skeleton className="h-12 w-full" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  if (error || !data) {
    return (
      <div className="text-center py-12">
        <p className="text-zinc-500 mb-4">Library not found.</p>
        <Link to="/" className="text-violet-400 hover:text-violet-300 text-sm">
          Back to libraries
        </Link>
      </div>
    );
  }

  return (
    <div>
      <Link
        to="/"
        className="inline-flex items-center gap-1 text-sm text-zinc-500 hover:text-zinc-300 mb-6 transition-colors"
      >
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M15 19l-7-7 7-7" />
        </svg>
        Back to libraries
      </Link>
      <LibraryDetail data={data} />
    </div>
  );
}
