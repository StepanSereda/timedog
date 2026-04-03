import type { ReportMeta, ChildDTO } from "./types";

const base = "";

/** Нативный диалог «Сохранить» на машине сервера (macOS / Linux+zenity). null = отмена. */
export async function browseOutputPath(suggested?: string) {
  const r = await fetch(`${base}/api/browse-output-path`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ suggested: suggested?.trim() || "timedog-report.jsonl" }),
  });
  if (r.status === 204) return null;
  if (r.status === 501) {
    throw new Error(await r.text());
  }
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<{ path: string }>;
}

export async function getSnapshots() {
  const r = await fetch(`${base}/api/snapshots`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function startScan(body: {
  old_root: string;
  new_root: string;
  output_path: string;
  options: Record<string, unknown>;
}) {
  const r = await fetch(`${base}/api/scan`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<{ id: string }>;
}

export async function scanStatus(id: string) {
  const r = await fetch(`${base}/api/scan/${id}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function cancelScan(id: string) {
  await fetch(`${base}/api/scan/${id}/cancel`, { method: "POST" });
}

export function scanEvents(
  id: string,
  onEvent: (ev: {
    type: string;
    progress?: number;
    message?: string;
    session_id?: string;
    skipped_total?: number;
    skipped_truncated?: boolean;
  }) => void
) {
  const url = `${base}/api/scan/${encodeURIComponent(id)}/events`;
  const es2 = new EventSource(url);
  es2.onmessage = (e) => {
    try {
      onEvent(JSON.parse(e.data));
    } catch {
      /* ignore */
    }
  };
  es2.onerror = () => es2.close();
  return () => es2.close();
}

export async function parseReport(file: File) {
  const fd = new FormData();
  fd.append("file", file);
  const r = await fetch(`${base}/api/reports/parse`, { method: "POST", body: fd });
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<{
    session_id: string;
    meta: ReportMeta;
    entry_count: number;
  }>;
}

export async function sessionMeta(sid: string) {
  const r = await fetch(`${base}/api/session/${sid}/meta`);
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<ReportMeta>;
}

export async function sessionSummary(sid: string) {
  const r = await fetch(`${base}/api/session/${sid}/summary`);
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<Record<string, number>>;
}

export async function sessionTree(sid: string, prefix: string, opts?: { filter?: string; q?: string }) {
  const q = new URLSearchParams({ prefix });
  if (opts?.filter) q.set("filter", opts.filter);
  if (opts?.q?.trim()) q.set("q", opts.q.trim());
  const r = await fetch(`${base}/api/session/${sid}/tree?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<{ prefix: string; children: ChildDTO[] }>;
}

export async function sessionContent(
  sid: string,
  path: string,
  mode: "text" | "hex",
  offset: number
) {
  const q = new URLSearchParams({ path, mode, offset: String(offset), limit: "131072" });
  const r = await fetch(`${base}/api/session/${sid}/content?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json() as Promise<{
    old?: Record<string, unknown>;
    new?: Record<string, unknown>;
  }>;
}

export async function deleteSession(sid: string) {
  await fetch(`${base}/api/session/${sid}`, { method: "DELETE" });
}
