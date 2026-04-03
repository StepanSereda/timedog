import { useEffect, useMemo, useState, type ReactNode } from "react";
import { useNavigate } from "react-router-dom";
import { HelpTip } from "./HelpTip";
import type { ReportMeta, Snapshot } from "./types";
import * as api from "./api";
import { SkippedPathsPanel } from "./SkippedPathsPanel";

const H = {
  oldSnap: `Каталог более раннего снимка Time Machine («эталон», сторона «старый»). Сравнение идёт как в timedog: что изменилось от этого бэкапа к новому. Путь берётся из tmutil; том с бэкапами должен быть смонтирован.`,
  newSnap: `Каталог более позднего снимка («новый»): текущий или более свежий бэкап относительно старого. Должен отличаться от старого снимка.`,
  outPath: `Полный путь к файлу отчёта на диске машины, где запущен timedog-server (не браузер). Формат: JSONL; суффикс .jsonl.gz — с gzip. Кнопка «Обзор…» открывает системный диалог сохранения на этой машине (macOS; на Linux — при установленном zenity). Без GUI или с удалённого сервера вводите путь вручную.`,
  depth: `Опция -d (глубина по сегментам пути от корня снимка): вместо перечисления каждого файла глубже уровня N изменения сворачиваются в одну строку на каталог с суммарными размерами и счётчиком (как в оригинальном timedog). Пусто = перечислить все пути.`,
  omitSymlinks: `Флаг -l: не включать симлинки в отчёт. У TM они часто «мигают» на каждом бэкапе и засоряют список.`,
  sortBy: `Флаг -S: порядок строк в отчёте после скана — по старому размеру, по новому размеру или по пути (имени).`,
  minMB: `Флаг -m: отбросить строки, где новый размер меньше указанного порога. Здесь ввод в мебибайтах (MiB); пусто = без ограничения.`,
  useBase10: `Флаг -H: показывать размеры в десятичных KB/MB/GB (1000), а не двоичных KiB/MiB (1024).`,
  simpleFormat: `Флаг -n: более простой числовой формат в колонках размеров (удобно копировать в таблицы), как в timedog -n.`,
  fastWalk: `Параллельный обход каталогов (fastwalk): быстрее на быстрых томах и SSD. На одном медленном внешнем HDD иногда выгоднее отключить и идти последовательно (меньше seek).`,
};

function RequiredMark() {
  return (
    <abbr className="required-mark" title="Обязательное поле">
      *
    </abbr>
  );
}

function RowHead({
  htmlFor,
  required,
  help,
  children,
}: {
  htmlFor: string;
  required?: boolean;
  help: string;
  children: ReactNode;
}) {
  return (
    <div className="row-head">
      <label htmlFor={htmlFor} className="field-label-main">
        {children}
        {required ? <RequiredMark /> : null}
      </label>
      <HelpTip text={help} />
    </div>
  );
}

export function ScanPage() {
  const nav = useNavigate();
  const [snaps, setSnaps] = useState<Snapshot[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [hint, setHint] = useState<string | null>(null);
  const [oldPath, setOldPath] = useState("");
  const [newPath, setNewPath] = useState("");
  const [outPath, setOutPath] = useState("");
  const [omitSymlinks, setOmitSymlinks] = useState(false);
  const [sortBy, setSortBy] = useState(2);
  const [depth, setDepth] = useState("");
  const [useBase10, setUseBase10] = useState(false);
  const [simpleFormat, setSimpleFormat] = useState(false);
  const [minMB, setMinMB] = useState("");
  const [fastWalk, setFastWalk] = useState(true);
  const [jobId, setJobId] = useState<string | null>(null);
  const [progress, setProgress] = useState<string>("");
  const [doneSession, setDoneSession] = useState<string | null>(null);
  const [outFile, setOutFile] = useState<string | null>(null);
  const [browsing, setBrowsing] = useState(false);
  const [doneMeta, setDoneMeta] = useState<ReportMeta | null>(null);

  useEffect(() => {
    api
      .getSnapshots()
      .then((list: Snapshot[]) => {
        setSnaps(list);
        if (list.length >= 2) {
          setOldPath(list[list.length - 2].path);
          setNewPath(list[list.length - 1].path);
        }
      })
      .catch((e) => setErr(String(e)));
  }, []);

  useEffect(() => {
    if (!doneSession) {
      setDoneMeta(null);
      return;
    }
    api
      .sessionMeta(doneSession)
      .then(setDoneMeta)
      .catch(() => setDoneMeta(null));
  }, [doneSession]);

  const validationMessage = useMemo(() => {
    if (snaps.length < 2) {
      return "Нужны минимум два снимка Time Machine (проверьте tmutil и доступ к диску).";
    }
    if (!oldPath.trim() || !newPath.trim()) {
      return "Выберите оба снимка.";
    }
    if (oldPath === newPath) {
      return "Старый и новый снимок должны быть разными каталогами.";
    }
    const o = outPath.trim();
    if (!o) {
      return "Укажите полный путь к файлу отчёта.";
    }
    const lower = o.toLowerCase();
    if (!lower.endsWith(".jsonl") && !lower.endsWith(".jsonl.gz")) {
      return "Путь отчёта лучше заканчивать на .jsonl или .jsonl.gz (сжатие по расширению).";
    }
    return null;
  }, [snaps.length, oldPath, newPath, outPath]);

  const canStart = validationMessage === null;

  const browseOut = async () => {
    setErr(null);
    setHint(null);
    setBrowsing(true);
    try {
      const sug = outPath.trim() || "timedog-report.jsonl";
      const res = await api.browseOutputPath(sug);
      if (res?.path) setOutPath(res.path);
    } catch (e) {
      setErr(String(e));
    } finally {
      setBrowsing(false);
    }
  };

  const start = async () => {
    setErr(null);
    setHint(null);
    if (validationMessage) {
      setHint(validationMessage);
      return;
    }
    setDoneSession(null);
    setDoneMeta(null);
    const options: Record<string, unknown> = {
      omit_symlinks: omitSymlinks,
      sort_by: sortBy,
      use_base10: useBase10,
      simple_format: simpleFormat,
    };
    if (depth.trim() !== "") {
      const d = parseInt(depth, 10);
      if (!Number.isNaN(d) && d > 0) options.depth = d;
    }
    if (minMB.trim() !== "") {
      const n = parseFloat(minMB) * 1024 * 1024;
      if (!Number.isNaN(n)) options.min_size_bytes = Math.floor(n);
    }
    if (!fastWalk) {
      options.fast_walk = false;
    }
    try {
      const { id } = await api.startScan({
        old_root: oldPath,
        new_root: newPath,
        output_path: outPath.trim(),
        options,
      });
      setJobId(id);
      setProgress("Запуск…");
      let cleanup: (() => void) | undefined;
      cleanup = api.scanEvents(id, (ev) => {
        if (ev.type === "progress") {
          setProgress(`Обработано узлов: ${ev.progress ?? 0}`);
        }
        if (ev.type === "done") {
          const sk = ev.skipped_total ?? 0;
          setProgress(
            sk > 0
              ? `Готово. Не обработано путей: ${sk}${ev.skipped_truncated ? " (список в отчёте усечён)" : ""}`
              : "Готово"
          );
          setDoneSession(ev.session_id ?? null);
          setOutFile(outPath.trim());
          cleanup?.();
        }
        if (ev.type === "error") {
          setErr(ev.message ?? "ошибка");
          cleanup?.();
        }
      });
    } catch (e) {
      setErr(String(e));
    }
  };

  const cancel = () => {
    if (jobId) api.cancelScan(jobId);
  };

  return (
    <div>
      <h1>Новый скан</h1>
      <p className="lead">
        Обязательные поля помечены <abbr className="required-mark inline-abbr" title="Обязательное поле">*</abbr>
        — наведите на <span className="help-preview">?</span> для пояснения опций.
      </p>
      {err && <div className="err">{err}</div>}
      {hint && <div className="hint-warn">{hint}</div>}
      {snaps.length < 2 && !err && (
        <div className="hint-warn">
          Снимки не загружены или их меньше двух. Убедитесь, что Time Machine настроена, диск с бэкапами смонтирован, и у приложения терминала есть «Полный доступ к диску».
        </div>
      )}
      <div className="panel">
        <h2 className="panel-title">Что сравниваем и куда сохранить</h2>
        <div className="row">
          <RowHead htmlFor="scan-old" required help={H.oldSnap}>
            Старый снимок (эталон)
          </RowHead>
          <select
            id="scan-old"
            value={oldPath}
            onChange={(e) => setOldPath(e.target.value)}
            required
            aria-required="true"
            disabled={snaps.length === 0}
          >
            {snaps.length === 0 ? <option value="">— нет снимков —</option> : null}
            {snaps.map((s) => (
              <option key={s.path} value={s.path}>
                {s.label} — {s.path}
              </option>
            ))}
          </select>
        </div>
        <div className="row">
          <RowHead htmlFor="scan-new" required help={H.newSnap}>
            Новый снимок
          </RowHead>
          <select
            id="scan-new"
            value={newPath}
            onChange={(e) => setNewPath(e.target.value)}
            required
            aria-required="true"
            disabled={snaps.length === 0}
          >
            {snaps.length === 0 ? <option value="">— нет снимков —</option> : null}
            {snaps.map((s) => (
              <option key={s.path} value={s.path}>
                {s.label} — {s.path}
              </option>
            ))}
          </select>
        </div>
        <div className="row">
          <RowHead htmlFor="scan-out" required help={H.outPath}>
            Путь к файлу отчёта
          </RowHead>
          <div className="path-row">
            <input
              id="scan-out"
              className="path-row-input"
              type="text"
              value={outPath}
              onChange={(e) => setOutPath(e.target.value)}
              placeholder="/Users/you/Desktop/tm-diff.jsonl"
              required
              aria-required="true"
              autoComplete="off"
              aria-invalid={
                outPath.trim() !== "" && !/\.jsonl(\.gz)?$/i.test(outPath.trim()) ? true : undefined
              }
            />
            <button
              type="button"
              className="secondary browse-btn"
              onClick={() => void browseOut()}
              disabled={browsing}
              title="Системный диалог на машине сервера (macOS / Linux+zenity)"
            >
              {browsing ? "…" : "Обзор…"}
            </button>
          </div>
          <p className="field-hint">Пример: <code className="inline-code">…/отчёт.jsonl</code> или <code className="inline-code">…/отчёт.jsonl.gz</code></p>
        </div>

        <h2 className="panel-title sub-title">
          Дополнительные опции
          <HelpTip
            text={`Все опции соответствуют флагам утилиты timedog. Их можно не трогать: по умолчанию — полный список путей, сортировка по имени, симлинки в отчёте, двоичные приставки размеров.`}
          />
        </h2>

        <div className="row">
          <RowHead htmlFor="scan-depth" help={H.depth}>
            Глубина сводки (-d), опционально
          </RowHead>
          <input
            id="scan-depth"
            type="text"
            inputMode="numeric"
            value={depth}
            onChange={(e) => setDepth(e.target.value)}
            placeholder="пусто = все файлы по отдельности"
          />
        </div>
        <div className="row row-check">
          <label className="check-label" htmlFor="scan-omitl">
            <input id="scan-omitl" type="checkbox" checked={omitSymlinks} onChange={(e) => setOmitSymlinks(e.target.checked)} />
            <span>Без символических ссылок (-l)</span>
            <HelpTip text={H.omitSymlinks} />
          </label>
        </div>
        <div className="row row-check">
          <label className="check-label" htmlFor="scan-fastwalk">
            <input id="scan-fastwalk" type="checkbox" checked={fastWalk} onChange={(e) => setFastWalk(e.target.checked)} />
            <span>Параллельный обход каталогов</span>
            <HelpTip text={H.fastWalk} />
          </label>
        </div>
        <div className="row">
          <RowHead htmlFor="scan-sort" help={H.sortBy}>
            Сортировка (-S)
          </RowHead>
          <select id="scan-sort" value={sortBy} onChange={(e) => setSortBy(Number(e.target.value))}>
            <option value={0}>По старому размеру</option>
            <option value={1}>По новому размеру</option>
            <option value={2}>По имени пути</option>
          </select>
        </div>
        <div className="row">
          <RowHead htmlFor="scan-minmb" help={H.minMB}>
            Минимум размера (МиБ), опционально (-m)
          </RowHead>
          <input id="scan-minmb" type="text" value={minMB} onChange={(e) => setMinMB(e.target.value)} placeholder="пусто = не фильтровать" />
        </div>
        <div className="row row-check">
          <label className="check-label" htmlFor="scan-h">
            <input id="scan-h" type="checkbox" checked={useBase10} onChange={(e) => setUseBase10(e.target.checked)} />
            <span>Размеры в KB/MB/GB по 1000 (-H)</span>
            <HelpTip text={H.useBase10} />
          </label>
        </div>
        <div className="row row-check">
          <label className="check-label" htmlFor="scan-n">
            <input id="scan-n" type="checkbox" checked={simpleFormat} onChange={(e) => setSimpleFormat(e.target.checked)} />
            <span>Простой формат чисел (-n)</span>
            <HelpTip text={H.simpleFormat} />
          </label>
        </div>
        <button type="button" onClick={start} disabled={!canStart} title={!canStart ? (validationMessage ?? "") : undefined}>
          Начать сканирование
        </button>
        <button type="button" className="secondary" onClick={cancel} disabled={!jobId}>
          Отменить
        </button>
        {!canStart && validationMessage && snaps.length > 0 && (
          <p className="field-hint muted">Скан не запущен: {validationMessage}</p>
        )}
      </div>
      {jobId && (
        <div className="panel">
          <div>Job: {jobId}</div>
          <div>{progress}</div>
          {doneSession && (
            <>
              <div className="ok">
                <p>Сохранено: {outFile}</p>
                <button type="button" onClick={() => nav(`/report?s=${encodeURIComponent(doneSession)}`)}>
                  Открыть этот отчёт
                </button>
              </div>
              {doneMeta && <SkippedPathsPanel meta={doneMeta} />}
            </>
          )}
        </div>
      )}
    </div>
  );
}
