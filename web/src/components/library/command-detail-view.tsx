import { useState, useEffect, useRef } from "react";
import yaml from "js-yaml";
import { Badge } from "../ui/badge";
import { Card } from "../ui/card";
import { CopyButton } from "../ui/copy-button";
import type { CommandDetail, CommandVersion, SpecJson } from "../../lib/api";

interface CommandDetailViewProps {
  data: CommandDetail;
  versions: CommandVersion[];
  librarySlug?: string;
}

function TemplateHighlighted({ text }: { text: string }) {
  const parts = text.split(/({{[^}]*}})/g);
  return (
    <>
      {parts.map((part, i) =>
        part.startsWith("{{") ? (
          <span key={i} className="text-violet-400">
            {part}
          </span>
        ) : (
          <span key={i}>{part}</span>
        )
      )}
    </>
  );
}

function UsageSection({
  spec,
  librarySlug,
}: {
  spec: SpecJson;
  librarySlug?: string;
}) {
  const parts = ["my"];
  if (librarySlug) parts.push(librarySlug);
  parts.push(spec.metadata.slug);

  if (spec.args?.positional) {
    for (const arg of spec.args.positional) {
      parts.push(arg.required !== false ? `<${arg.name}>` : `[${arg.name}]`);
    }
  }
  if (spec.args?.flags) {
    for (const flag of spec.args.flags) {
      if (flag.type === "bool") {
        parts.push(`[--${flag.name}]`);
      } else {
        parts.push(`[--${flag.name} <value>]`);
      }
    }
  }

  const usage = parts.join(" ");

  return (
    <div className="flex items-center gap-2 rounded-lg bg-zinc-950 border border-zinc-800 px-4 py-2.5">
      <code className="font-mono text-sm text-emerald-400 flex-1 overflow-x-auto">
        $ {usage}
      </code>
      <CopyButton text={usage} />
    </div>
  );
}

function ArgumentsSection({ spec }: { spec: SpecJson }) {
  const positional = spec.args?.positional ?? [];
  const flags = spec.args?.flags ?? [];
  if (positional.length === 0 && flags.length === 0) return null;

  return (
    <Card>
      <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
        Arguments
      </h2>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-left text-zinc-500 border-b border-zinc-800">
              <th className="pb-2 pr-4 font-medium">Argument</th>
              <th className="pb-2 pr-4 font-medium">Type</th>
              <th className="pb-2 pr-4 font-medium">Required</th>
              <th className="pb-2 pr-4 font-medium">Default</th>
              <th className="pb-2 font-medium">Description</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800/50">
            {positional.map((arg) => (
              <tr key={arg.name} className="text-zinc-300">
                <td className="py-2 pr-4 font-mono text-zinc-100">
                  {arg.name}
                </td>
                <td className="py-2 pr-4">
                  <Badge>positional</Badge>
                </td>
                <td className="py-2 pr-4">
                  {arg.required !== false ? (
                    <Badge variant="info">required</Badge>
                  ) : (
                    <Badge>optional</Badge>
                  )}
                </td>
                <td className="py-2 pr-4 font-mono text-zinc-500">
                  {arg.default ?? "—"}
                </td>
                <td className="py-2 text-zinc-400">
                  {arg.description ?? "—"}
                </td>
              </tr>
            ))}
            {flags.map((flag) => (
              <tr key={flag.name} className="text-zinc-300">
                <td className="py-2 pr-4 font-mono text-zinc-100">
                  --{flag.name}
                  {flag.short && (
                    <span className="text-zinc-500">, -{flag.short}</span>
                  )}
                </td>
                <td className="py-2 pr-4">
                  <Badge>{flag.type ?? "string"}</Badge>
                </td>
                <td className="py-2 pr-4">
                  {flag.required ? (
                    <Badge variant="info">required</Badge>
                  ) : (
                    <Badge>optional</Badge>
                  )}
                </td>
                <td className="py-2 pr-4 font-mono text-zinc-500">
                  {flag.default !== undefined ? String(flag.default) : "—"}
                </td>
                <td className="py-2 text-zinc-400">
                  {flag.description ?? "—"}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Card>
  );
}

function StepsSection({ spec }: { spec: SpecJson }) {
  if (!spec.steps || spec.steps.length === 0) return null;

  return (
    <Card>
      <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
        Steps
      </h2>
      <div className="space-y-3">
        {spec.steps.map((step, i) => (
          <div key={i} className="rounded-lg bg-zinc-950 border border-zinc-800 overflow-hidden">
            <div className="flex items-center gap-2 px-4 py-2 border-b border-zinc-800/50">
              <span className="text-xs font-mono text-zinc-500">
                {i + 1}
              </span>
              <span className="text-sm font-medium text-zinc-200">
                {step.name}
              </span>
              {step.shell && (
                <Badge className="ml-auto">{step.shell}</Badge>
              )}
              {step.timeout && <Badge>{step.timeout}</Badge>}
              {step.continueOnError && (
                <Badge variant="info">continue on error</Badge>
              )}
            </div>
            <pre className="px-4 py-3 text-sm font-mono text-emerald-400 overflow-x-auto">
              <TemplateHighlighted text={step.run} />
            </pre>
            {step.env && Object.keys(step.env).length > 0 && (
              <div className="px-4 py-2 border-t border-zinc-800/50">
                <span className="text-xs text-zinc-500 mr-2">env:</span>
                {Object.entries(step.env).map(([k, v]) => (
                  <span
                    key={k}
                    className="inline-block font-mono text-xs text-zinc-400 mr-3"
                  >
                    {k}=<span className="text-zinc-300">{v}</span>
                  </span>
                ))}
              </div>
            )}
          </div>
        ))}
      </div>
    </Card>
  );
}

function ConfigurationSection({ spec }: { spec: SpecJson }) {
  const hasDefaults = spec.defaults && Object.keys(spec.defaults).length > 0;
  const hasDeps = spec.dependencies && spec.dependencies.length > 0;
  const hasPolicy = spec.policy && Object.keys(spec.policy).length > 0;
  if (!hasDefaults && !hasDeps && !hasPolicy) return null;

  return (
    <Card>
      <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
        Configuration
      </h2>
      <div className="space-y-4">
        {spec.defaults?.shell && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-zinc-500 w-24 shrink-0">Shell</span>
            <Badge>{spec.defaults.shell}</Badge>
          </div>
        )}
        {spec.defaults?.timeout && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-zinc-500 w-24 shrink-0">
              Timeout
            </span>
            <Badge>{spec.defaults.timeout}</Badge>
          </div>
        )}
        {spec.defaults?.env &&
          Object.keys(spec.defaults.env).length > 0 && (
            <div>
              <span className="text-sm text-zinc-500">
                Environment Variables
              </span>
              <div className="mt-1.5 rounded-lg bg-zinc-950 border border-zinc-800 px-4 py-2.5 font-mono text-sm overflow-x-auto">
                {Object.entries(spec.defaults.env).map(([k, v]) => (
                  <div key={k} className="text-zinc-400">
                    <span className="text-zinc-300">{k}</span>={v}
                  </div>
                ))}
              </div>
            </div>
          )}
        {hasDeps && (
          <div>
            <span className="text-sm text-zinc-500">Dependencies</span>
            <div className="flex gap-2 flex-wrap mt-1.5">
              {spec.dependencies!.map((dep) => (
                <Badge key={dep}>{dep}</Badge>
              ))}
            </div>
          </div>
        )}
        {hasPolicy && (
          <div>
            <span className="text-sm text-zinc-500">Policy</span>
            <div className="flex gap-2 flex-wrap mt-1.5">
              {spec.policy!.requireConfirmation && (
                <Badge variant="info">requires confirmation</Badge>
              )}
              {spec.policy!.allowedExecutables?.map((exe) => (
                <Badge key={exe}>{exe}</Badge>
              ))}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}

function RawSpecSection({ spec }: { spec: SpecJson }) {
  const [open, setOpen] = useState(false);
  const [format, setFormat] = useState<"yaml" | "json">("yaml");
  const [highlighted, setHighlighted] = useState<string | null>(null);
  const highlighterRef = useRef<Awaited<
    ReturnType<typeof import("shiki")["createHighlighter"]>
  > | null>(null);

  const raw =
    format === "yaml"
      ? yaml.dump(spec, { lineWidth: 120, noRefs: true })
      : JSON.stringify(spec, null, 2);

  useEffect(() => {
    if (!open) return;

    let cancelled = false;

    (async () => {
      if (!highlighterRef.current) {
        const { createHighlighter } = await import("shiki");
        highlighterRef.current = await createHighlighter({
          themes: ["github-dark"],
          langs: ["yaml", "json"],
        });
      }
      if (cancelled) return;
      const html = highlighterRef.current.codeToHtml(raw, {
        lang: format,
        theme: "github-dark",
      });
      setHighlighted(html);
    })();

    return () => {
      cancelled = true;
    };
  }, [open, raw, format]);

  return (
    <div>
      <button
        onClick={() => setOpen(!open)}
        className="flex items-center gap-2 text-sm text-zinc-500 hover:text-zinc-300 transition-colors cursor-pointer"
      >
        <svg
          className={`w-4 h-4 transition-transform ${open ? "rotate-90" : ""}`}
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M9 5l7 7-7 7"
          />
        </svg>
        View raw spec
      </button>
      {open && (
        <div className="mt-3">
          <div className="flex items-center gap-1 mb-2">
            <button
              onClick={() => setFormat("yaml")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors cursor-pointer ${
                format === "yaml"
                  ? "bg-zinc-700 text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-300"
              }`}
            >
              YAML
            </button>
            <button
              onClick={() => setFormat("json")}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors cursor-pointer ${
                format === "json"
                  ? "bg-zinc-700 text-zinc-100"
                  : "text-zinc-500 hover:text-zinc-300"
              }`}
            >
              JSON
            </button>
            <div className="flex-1" />
            <CopyButton text={raw} />
          </div>
          {highlighted ? (
            <div
              className="text-sm [&_pre]:!bg-zinc-950 [&_pre]:border [&_pre]:border-zinc-800 [&_pre]:p-4 [&_pre]:rounded-lg [&_pre]:overflow-x-auto [&_pre]:max-h-[600px] [&_pre]:overflow-y-auto"
              dangerouslySetInnerHTML={{ __html: highlighted }}
            />
          ) : (
            <pre className="text-sm font-mono text-zinc-300 bg-zinc-950 border border-zinc-800 rounded-lg p-4 overflow-x-auto max-h-[600px] overflow-y-auto">
              {raw}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

export function CommandDetailView({
  data,
  versions,
  librarySlug,
}: CommandDetailViewProps) {
  const { command, latest_version } = data;
  const [selectedVersion, setSelectedVersion] = useState<CommandVersion | null>(
    latest_version ?? versions[0] ?? null
  );

  const spec = selectedVersion?.spec_json ?? null;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <div className="flex items-center gap-2 mb-2">
          <span className="font-mono font-bold text-2xl text-zinc-100">
            {command.slug}
          </span>
          {selectedVersion && (
            <Badge variant="default">v{selectedVersion.version}</Badge>
          )}
        </div>
        {command.name !== command.slug && (
          <p className="text-zinc-300 mb-1">{command.name}</p>
        )}
        {command.description && (
          <p className="text-zinc-400 mb-3">{command.description}</p>
        )}
        {command.tags && command.tags.length > 0 && (
          <div className="flex gap-2 flex-wrap">
            {command.tags.map((tag) => (
              <Badge key={tag} variant="default">
                {tag}
              </Badge>
            ))}
          </div>
        )}
      </div>

      {/* Usage */}
      {spec && <UsageSection spec={spec} librarySlug={librarySlug} />}

      {/* Arguments */}
      {spec && <ArgumentsSection spec={spec} />}

      {/* Steps */}
      {spec && <StepsSection spec={spec} />}

      {/* Configuration */}
      {spec && <ConfigurationSection spec={spec} />}

      {/* Raw spec toggle */}
      {spec && <RawSpecSection spec={spec} />}

      {/* Version history */}
      {versions.length > 0 && (
        <Card>
          <h2 className="text-sm font-medium text-zinc-400 mb-3 uppercase tracking-wider">
            Version History ({versions.length})
          </h2>
          <div className="divide-y divide-zinc-800">
            {versions.map((v) => {
              const isSelected = selectedVersion?.id === v.id;
              return (
                <button
                  key={v.id}
                  onClick={() => setSelectedVersion(v)}
                  className={`w-full flex items-center gap-3 py-3 first:pt-0 last:pb-0 text-left transition-colors ${
                    isSelected
                      ? "text-zinc-100"
                      : "text-zinc-400 hover:text-zinc-200"
                  }`}
                >
                  <Badge variant={isSelected ? "info" : "default"}>
                    v{v.version}
                  </Badge>
                  <span className="text-sm truncate flex-1">
                    {v.message || "No message"}
                  </span>
                  <span className="text-xs text-zinc-600 shrink-0">
                    {new Date(v.created_at).toLocaleDateString()}
                  </span>
                </button>
              );
            })}
          </div>
        </Card>
      )}
    </div>
  );
}
