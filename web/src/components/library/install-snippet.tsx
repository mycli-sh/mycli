import { CopyButton } from "../ui/copy-button";
import { isSystemOwner } from "../../lib/owner-utils";

export function InstallSnippet({ owner, slug }: { owner: string; slug: string }) {
  const command = isSystemOwner(owner)
    ? `my library add ${slug}`
    : `my library add ${owner}/${slug}`;

  return (
    <div className="flex items-center gap-2 rounded-lg bg-zinc-950 border border-zinc-800 px-4 py-2.5">
      <code className="font-mono text-sm text-emerald-400 flex-1">
        $ {command}
      </code>
      <CopyButton text={command} />
    </div>
  );
}
