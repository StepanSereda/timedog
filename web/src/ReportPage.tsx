import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import type { ChildDTO, ReportEntry, ReportMeta } from "./types";
import * as api from "./api";
import { ContentDiffModal } from "./ContentDiffModal";
import { SkippedPathsPanel } from "./SkippedPathsPanel";

function clsEntry(e?: ReportEntry | null) {
  if (!e) return "same";
  if (e.unknown_old) return "rem";
  const o = e.old_size,
    n = e.new_size;
  if (o === 0 && n === 0) return "same";
  if (o === 0 && n > 0) return "new";
  if (o > 0 && n === 0) return "rem";
  if (n > o) return "inc";
  if (n < o) return "dec";
  return "same";
}

export function ReportPage() {
  const [params, setParams] = useSearchParams();
  const [sid, setSid] = useState<string | null>(params.get("s"));
  const [meta, setMeta] = useState<ReportMeta | null>(null);
  const [summary, setSummary] = useState<Record<string, number> | null>(null);
  const [err, setErr] = useState<string | null>(null);
  /** Выбранный файл в дереве + строка отчёта (для сверки с чтением с диска в модалке). */
  const [selectedFile, setSelectedFile] = useState<{
    path: string;
    entry: ReportEntry | null;
  } | null>(null);
  const [chipFilter, setChipFilter] = useState<string>("all");
  const [search, setSearch] = useState("");

  useEffect(() => {
    const s = params.get("s");
    if (s) setSid(s);
  }, [params]);

  useEffect(() => {
    if (!sid) return;
    api
      .sessionMeta(sid)
      .then(setMeta)
      .catch((e) => setErr(String(e)));
    api
      .sessionSummary(sid)
      .then(setSummary)
      .catch(() => {});
  }, [sid]);

  const onFile = async (f: File) => {
    setErr(null);
    try {
      const res = await api.parseReport(f);
      setSid(res.session_id);
      setParams({ s: res.session_id });
      setMeta(res.meta);
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <div>
      <h1>Открыть отчёт</h1>
      {err && <div className="err">{err}</div>}
      {!sid && (
        <div className="panel">
          <input
            type="file"
            accept=".jsonl,.jsonl.gz,.json,.txt,application/gzip"
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) void onFile(f);
            }}
          />
        </div>
      )}
      {meta && (
        <div className="panel">
          <div>
            <strong>
              {meta.new_label} → {meta.old_label}
            </strong>
          </div>
          <div style={{ fontSize: "0.8rem", color: "var(--dim)", wordBreak: "break-all" }}>
            new: {meta.new_root}
            <br />
            old: {meta.old_root}
          </div>
          {summary && (
            <div className="chips">
              <button
                type="button"
                className={`chip chip-all ${chipFilter === "all" ? "on" : ""}`}
                onClick={() => setChipFilter("all")}
              >
                всего {summary.total}
              </button>
              <button
                type="button"
                className={`chip chip-same ${chipFilter === "same" ? "on" : ""}`}
                onClick={() => setChipFilter("same")}
              >
                без изм. {summary.same}
              </button>
              <button
                type="button"
                className={`chip chip-changed ${chipFilter === "changed" ? "on" : ""}`}
                onClick={() => setChipFilter("changed")}
              >
                измен. {summary.changed}
              </button>
              <button
                type="button"
                className={`chip chip-new ${chipFilter === "new" ? "on" : ""}`}
                onClick={() => setChipFilter("new")}
              >
                нов. {summary.new}
              </button>
              <button
                type="button"
                className={`chip chip-removed ${chipFilter === "removed" ? "on" : ""}`}
                onClick={() => setChipFilter("removed")}
              >
                удал. {summary.removed}
              </button>
            </div>
          )}
          <div className="row" style={{ marginTop: 12 }}>
            <label>Поиск по пути / имени</label>
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="фрагмент пути…"
            />
          </div>
          <SkippedPathsPanel meta={meta} />
          <button
            type="button"
            className="secondary"
            onClick={() => {
              if (sid) api.deleteSession(sid);
              setSid(null);
              setMeta(null);
              setParams({});
              setSelectedFile(null);
            }}
          >
            Закрыть отчёт
          </button>
        </div>
      )}
      {sid && meta && (
        <>
          {!meta.old_root?.trim() || !meta.new_root?.trim() ? (
            <div className="panel err">
              В метаданных отчёта нет корней снимков на диске — дерево доступно, просмотр содержимого файлов нет.
            </div>
          ) : null}
          <div className="panel tree tree-legend">
            <p className="tree-legend-line">
              <span className="tree-legend-item">
                <span className="lg lg-new" aria-hidden />
                новый
              </span>
              <span className="tree-legend-item">
                <span className="lg lg-rem" aria-hidden />
                удалён / нет старого
              </span>
              <span className="tree-legend-item">
                <span className="lg lg-chg" aria-hidden />
                изменён размер
              </span>
              <span className="tree-legend-item">
                <span className="lg lg-same" aria-hidden />
                без изменений
              </span>
            </p>
            <TreeRoot
              sessionId={sid}
              chipFilter={chipFilter}
              search={search}
              selectedPath={selectedFile?.path ?? null}
              onPick={(path, isDir, entry) => {
                if (!isDir) {
                  setSelectedFile({ path, entry: entry ?? null });
                }
              }}
            />
          </div>
          <ContentDiffModal
            open={!!selectedFile}
            sessionId={sid}
            path={selectedFile?.path ?? null}
            reportEntry={selectedFile?.entry ?? null}
            hasRoots={!!(meta.old_root?.trim() && meta.new_root?.trim())}
            onClose={() => setSelectedFile(null)}
          />
        </>
      )}
    </div>
  );
}

function TreeRoot({
  sessionId,
  chipFilter,
  search,
  selectedPath,
  onPick,
}: {
  sessionId: string;
  chipFilter: string;
  search: string;
  selectedPath: string | null;
  onPick: (path: string, isDir: boolean, entry?: ReportEntry | null) => void;
}) {
  return (
    <div className="tree-scroll">
      <div className="tree-header" role="row">
        <span className="tree-h-name">Путь</span>
        <span className="tree-h-old" title="Отображаемая величина на старом снимке">
          Старый
        </span>
        <span className="tree-h-new" title="Отображаемая величина на новом снимке">
          Новый
        </span>
        <span className="tree-h-delta" title="Разница новый − старый (байт); для сводки -d — число вложенных изменений">
          Δ
        </span>
      </div>
      <TreeChildren
        sessionId={sessionId}
        prefix="/"
        depth={0}
        chipFilter={chipFilter}
        search={search}
        selectedPath={selectedPath}
        onPick={onPick}
      />
    </div>
  );
}

function TreeChildren({
  sessionId,
  prefix,
  depth,
  chipFilter,
  search,
  selectedPath,
  onPick,
}: {
  sessionId: string;
  prefix: string;
  depth: number;
  chipFilter: string;
  search: string;
  selectedPath: string | null;
  onPick: (path: string, isDir: boolean, entry?: ReportEntry | null) => void;
}) {
  const [nodes, setNodes] = useState<ChildDTO[] | null>(null);
  const [open, setOpen] = useState<Record<string, boolean>>({});

  useEffect(() => {
    api
      .sessionTree(sessionId, prefix, {
        filter: chipFilter === "all" ? undefined : chipFilter,
        q: search,
      })
      .then((r) => setNodes(r.children));
  }, [sessionId, prefix, chipFilter, search]);

  if (!nodes) return <div className="tree-loading">Загрузка…</div>;

  return (
    <>
      {nodes.map((n) => {
        const e = n.entry;
        const b = "b-" + clsEntry(e || undefined);
        const pad = depth * 16;
        return (
          <div key={n.path}>
            <div
              className={`tree-row ${n.is_dir ? "dir" : ""} ${b} ${selectedPath === n.path ? "selected" : ""}`}
              onClick={() => {
                if (n.is_dir) {
                  setOpen((o) => ({ ...o, [n.path]: !o[n.path] }));
                } else {
                  onPick(n.path, false, e ?? null);
                }
              }}
            >
              <span className="tree-name" style={{ paddingLeft: pad }}>
                {n.is_dir ? (open[n.path] ? "▼ " : "▶ ") : ""}
                {n.is_dir ? "📁 " : "📄 "}
                {n.name}
              </span>
              <span>{e?.old_str ?? "—"}</span>
              <span>{e?.new_str ?? "—"}</span>
              <span>
                {e?.in_dir != null && e.in_dir > 1 ? `[${e.in_dir - 1}] ` : ""}
                {e && !e.unknown_old ? `${e.new_size - e.old_size}` : "…"}
              </span>
            </div>
            {n.is_dir && open[n.path] && (
              <TreeChildren
                sessionId={sessionId}
                prefix={n.path}
                depth={depth + 1}
                chipFilter={chipFilter}
                search={search}
                selectedPath={selectedPath}
                onPick={onPick}
              />
            )}
          </div>
        );
      })}
    </>
  );
}
