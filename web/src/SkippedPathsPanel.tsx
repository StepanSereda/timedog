import { useState } from "react";
import type { ReportMeta, SkippedPath } from "./types";

const REASON_HINT: Record<string, string> = {
  walk: "нет доступа при обходе каталога (readdir/вход)",
  info: "не удалось прочитать метаданные записи",
  lstat_access: "запрещён lstat (часто права / TCC)",
  lstat: "ошибка lstat на новом снимке",
};

function reasonLabel(r: string) {
  return REASON_HINT[r] ?? r;
}

export function SkippedPathsPanel({ meta }: { meta: ReportMeta }) {
  const total = meta.skipped_total ?? 0;
  const list = meta.skipped ?? [];
  const truncated = meta.skipped_truncated ?? false;

  const [open, setOpen] = useState(false);

  if (total === 0 && list.length === 0) return null;

  return (
    <div className="skipped-panel">
      <button type="button" className="skipped-toggle" onClick={() => setOpen((o) => !o)}>
        <span>
          Не обработано при скане: <strong>{total}</strong> путей
          {truncated ? " (в интерфейсе и в meta отчёта сохраняются первые 5000)" : ""}
        </span>
        <span className="skipped-chev">{open ? "▼" : "▶"}</span>
      </button>
      {open && list.length > 0 && (
        <>
          <p className="field-hint skipped-legend">
            Пути относительно корня <strong>нового</strong> снимка. Код:{" "}
            <code>walk</code> — обход, <code>info</code> — метаданные, <code>lstat_access</code> /{" "}
            <code>lstat</code> — чтение inode.
          </p>
          <div className="skipped-table-wrap">
            <table className="skipped-table">
              <thead>
                <tr>
                  <th>Путь</th>
                  <th>Причина</th>
                </tr>
              </thead>
              <tbody>
                {list.map((row: SkippedPath, i: number) => (
                  <tr key={`${row.path}-${row.reason}-${i}`}>
                    <td className="skipped-path">{row.path}</td>
                    <td title={reasonLabel(row.reason)}>
                      <code>{row.reason}</code>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
      {open && list.length === 0 && total > 0 && (
        <p className="field-hint">Подробный список есть в первой строке метаданных файла отчёта (поле skipped), либо откройте отчёт после полной пересборки.</p>
      )}
    </div>
  );
}
