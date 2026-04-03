import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactElement,
} from "react";
import ReactDiffViewer, { DiffMethod } from "react-diff-viewer-continued";
import * as api from "./api";
import type { ReportEntry } from "./types";

function decodeB64(s?: string): Uint8Array {
  if (!s) return new Uint8Array(0);
  try {
    const bin = atob(s);
    const u = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) u[i] = bin.charCodeAt(i);
    return u;
  } catch {
    return new Uint8Array(0);
  }
}

function readSideText(side: unknown): string {
  if (!side || typeof side !== "object") return "";
  const o = side as Record<string, unknown>;
  if (typeof o.error === "string") return "";
  if (o.is_dir === true) return "";
  return typeof o.text === "string" ? o.text : "";
}

function readSideErr(side: unknown): string | null {
  if (!side || typeof side !== "object") return null;
  const o = side as Record<string, unknown>;
  return typeof o.error === "string" ? o.error : null;
}

/** Полный размер файла на снимке (из ответа /content). */
function readSideFileSize(side: unknown): number | undefined {
  if (!side || typeof side !== "object") return undefined;
  const o = side as Record<string, unknown>;
  const n = o.size;
  return typeof n === "number" && Number.isFinite(n) ? n : undefined;
}

const CONTENT_SLICE_LIMIT = 131072;

function tailContentOffset(oldSz: number, newSz: number): number {
  return Math.max(0, Math.max(oldSz, newSz) - CONTENT_SLICE_LIMIT);
}

/** Если размеры по stat при /content не совпали с JSONL — баг среды или отчёт устарел. */
function reportDiskMismatchMessage(
  report: ReportEntry | null | undefined,
  oldDisk?: number,
  newDisk?: number
): string | null {
  if (!report || oldDisk == null || newDisk == null) return null;
  const bits: string[] = [];
  if (!report.unknown_old && oldDisk !== report.old_size) {
    bits.push(`старый снимок: в отчёте ${report.old_size} Б, сейчас на диске ${oldDisk} Б`);
  }
  if (newDisk !== report.new_size) {
    bits.push(`новый снимок: в отчёте ${report.new_size} Б, сейчас на диске ${newDisk} Б`);
  }
  if (bits.length === 0) return null;
  return bits.join("; ") + ". Возможны другой открытый отчёт, смещённые корни или изменение файла после скана.";
}

function readSideB64(side: unknown): Uint8Array {
  if (!side || typeof side !== "object") return new Uint8Array(0);
  const o = side as Record<string, unknown>;
  if (typeof o.error === "string") return new Uint8Array(0);
  return decodeB64(typeof o.raw_b64 === "string" ? o.raw_b64 : undefined);
}

/**
 * Две колонки + подсветка символов. Параметр showDiffOnly — встроенная в react-diff-viewer-continued
 * свёртка неизменённых блоков слева и справа (развернуть блок: клик по строке «@@ … @@»).
 */
function TextDiffViewerPane({
  oldStr,
  newStr,
  showDiffOnly,
}: {
  oldStr: string;
  newStr: string;
  showDiffOnly: boolean;
}) {
  return (
    <div className="text-diff-viewer-wrap">
      <ReactDiffViewer
        oldValue={oldStr || " "}
        newValue={newStr || " "}
        splitView
        useDarkTheme
        compareMethod={DiffMethod.CHARS}
        disableWordDiff={false}
        showDiffOnly={showDiffOnly}
        hideLineNumbers={false}
        hideSummary
        leftTitle="Предыдущий снимок (старый)"
        rightTitle="Новый снимок"
        extraLinesSurroundingDiff={4}
      />
    </div>
  );
}

function hexRowAllBytesEqual(rowStart: number, oldBuf: Uint8Array, newBuf: Uint8Array, maxLen: number): boolean {
  for (let j = 0; j < 16; j++) {
    const idx = rowStart + j;
    if (idx >= maxLen) break;
    const ob = idx < oldBuf.length ? oldBuf[idx] : null;
    const nb = idx < newBuf.length ? newBuf[idx] : null;
    if (ob !== nb) return false;
  }
  return true;
}

function hexRowStarts(oldBuf: Uint8Array, newBuf: Uint8Array, hideEqualRows: boolean): number[] {
  const maxLen = Math.max(oldBuf.length, newBuf.length);
  const starts: number[] = [];
  for (let rowStart = 0; rowStart < maxLen; rowStart += 16) {
    if (!hideEqualRows || !hexRowAllBytesEqual(rowStart, oldBuf, newBuf, maxLen)) {
      starts.push(rowStart);
    }
  }
  return starts;
}

function renderHexRows(
  rowStarts: number[],
  oldBuf: Uint8Array,
  newBuf: Uint8Array,
  baseOffset: number,
  side: "old" | "new"
): ReactElement[] {
  const maxLen = Math.max(oldBuf.length, newBuf.length);
  const rows: ReactElement[] = [];
  for (const rowStart of rowStarts) {
    const cells: ReactElement[] = [];
    for (let j = 0; j < 16; j++) {
      const idx = rowStart + j;
      if (idx >= maxLen) break;
      const ob = idx < oldBuf.length ? oldBuf[idx] : null;
      const nb = idx < newBuf.length ? newBuf[idx] : null;
      let cls = "hex-byte";
      let disp = "  ";
      let title = `offset ${baseOffset + idx}`;
      if (side === "old") {
        if (ob === null) {
          cls += " hex-byte-pad";
          disp = "  ";
          title = nb !== null ? `нет байта в старом; в новом: ${nb.toString(16).toUpperCase().padStart(2, "0")}` : title;
        } else if (nb === null) {
          cls += " hex-byte-del";
          disp = ob.toString(16).toUpperCase().padStart(2, "0");
          title = `${title} — только в старом`;
        } else if (ob !== nb) {
          cls += " hex-byte-diff";
          disp = ob.toString(16).toUpperCase().padStart(2, "0");
          title = `${title}: в новом ${nb.toString(16).toUpperCase().padStart(2, "0")}`;
        } else {
          cls += " hex-byte-eq";
          disp = ob.toString(16).toUpperCase().padStart(2, "0");
        }
      } else {
        if (nb === null) {
          cls += " hex-byte-pad";
          disp = "  ";
          title = ob !== null ? `нет байта в новом; в старом: ${ob.toString(16).toUpperCase().padStart(2, "0")}` : title;
        } else if (ob === null) {
          cls += " hex-byte-ins";
          disp = nb.toString(16).toUpperCase().padStart(2, "0");
          title = `${title} — только в новом`;
        } else if (ob !== nb) {
          cls += " hex-byte-diff";
          disp = nb.toString(16).toUpperCase().padStart(2, "0");
          title = `${title}: было ${ob.toString(16).toUpperCase().padStart(2, "0")}`;
        } else {
          cls += " hex-byte-eq";
          disp = nb.toString(16).toUpperCase().padStart(2, "0");
        }
      }
      cells.push(
        <span key={idx} className={cls} title={title}>
          {disp}
        </span>
      );
    }
    rows.push(
      <div key={rowStart} className="hex-row">
        <span className="hex-addr">{(baseOffset + rowStart).toString(16).toUpperCase().padStart(8, "0")}</span>
        <span className="hex-cells">{cells}</span>
      </div>
    );
  }
  return rows;
}

function HexSplitView({
  oldBuf,
  newBuf,
  baseOffset,
  hideEqualRows,
  oldFileSize,
  newFileSize,
}: {
  oldBuf: Uint8Array;
  newBuf: Uint8Array;
  baseOffset: number;
  hideEqualRows: boolean;
  oldFileSize?: number;
  newFileSize?: number;
}) {
  const leftScrollRef = useRef<HTMLDivElement>(null);
  const rightScrollRef = useRef<HTMLDivElement>(null);
  const syncLock = useRef(false);

  const maxLen = Math.max(oldBuf.length, newBuf.length);
  const rowStarts = useMemo(
    () => hexRowStarts(oldBuf, newBuf, hideEqualRows),
    [oldBuf, newBuf, hideEqualRows]
  );

  useEffect(() => {
    const L = leftScrollRef.current;
    const R = rightScrollRef.current;
    if (!L || !R) return;
    const mirror = (source: HTMLDivElement, target: HTMLDivElement) => {
      if (syncLock.current) return;
      syncLock.current = true;
      target.scrollTop = source.scrollTop;
      target.scrollLeft = source.scrollLeft;
      requestAnimationFrame(() => {
        syncLock.current = false;
      });
    };
    const onL = () => mirror(L, R);
    const onR = () => mirror(R, L);
    L.addEventListener("scroll", onL, { passive: true });
    R.addEventListener("scroll", onR, { passive: true });
    return () => {
      L.removeEventListener("scroll", onL);
      R.removeEventListener("scroll", onR);
    };
  }, []);

  useEffect(() => {
    const L = leftScrollRef.current;
    const R = rightScrollRef.current;
    if (L) L.scrollTop = 0;
    if (R) R.scrollTop = 0;
  }, [oldBuf.length, newBuf.length, baseOffset, hideEqualRows]);

  if (maxLen === 0) {
    return <p className="content-modal-foot">В этом фрагменте нет байт (оба среза нулевой длины).</p>;
  }

  if (hideEqualRows && rowStarts.length === 0) {
    const sizesDiffer =
      oldFileSize != null &&
      newFileSize != null &&
      oldFileSize !== newFileSize;
    return (
      <div className="content-hex-empty-hint">
        <p className="content-modal-foot">
          {sizesDiffer
            ? "В этом фрагменте hex построчно совпадает, хотя в дереве у файла разные размеры на снимках — типично для дозаписи в конец (например .zsh_history): начало файла одинаковое, отличие дальше. Снимите «Скрыть неизменённые» или нажмите «К концу файла» в шапке."
            : "В этом фрагменте при свёртке не осталось строк с отличающимися байтами. Снимите флажок «Скрыть неизменённые», чтобы показать весь hex."}
        </p>
      </div>
    );
  }

  return (
    <div className="hex-merge-wrap">
      <p className="hex-merge-legend">
        <span className="hex-byte-eq hex-legend-swatch">совпадает</span>
        <span className="hex-byte-diff hex-legend-swatch">другой байт</span>
        <span className="hex-byte-ins hex-legend-swatch">только справа (новый)</span>
        <span className="hex-byte-del hex-legend-swatch">только слева (старый)</span>
        <span className="hex-byte-pad hex-legend-swatch">пусто на этой стороне</span>
      </p>
      <div className="content-split content-split-hex">
        <div className="content-split-col">
          <div className="content-split-col-head">Предыдущий снимок (старый)</div>
          <div ref={leftScrollRef} className="hex-pane-scroll">
            {renderHexRows(rowStarts, oldBuf, newBuf, baseOffset, "old")}
          </div>
        </div>
        <div className="content-split-col">
          <div className="content-split-col-head">Новый снимок</div>
          <div ref={rightScrollRef} className="hex-pane-scroll">
            {renderHexRows(rowStarts, oldBuf, newBuf, baseOffset, "new")}
          </div>
        </div>
      </div>
    </div>
  );
}

export function ContentDiffModal({
  open,
  sessionId,
  path,
  reportEntry,
  hasRoots,
  onClose,
}: {
  open: boolean;
  sessionId: string | null;
  path: string | null;
  /** Строка из дерева (тот же JSONL), чтобы сверить размеры с live stat в API. */
  reportEntry?: ReportEntry | null;
  hasRoots: boolean;
  onClose: () => void;
}) {
  const [mode, setMode] = useState<"text" | "hex">("text");
  /** Текст: showDiffOnly у react-diff-viewer. Hex: только строки с отличающимися байтами. */
  const [hideUnchanged, setHideUnchanged] = useState(false);
  const [contentOffset, setContentOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [fetchErr, setFetchErr] = useState<string | null>(null);
  const [payload, setPayload] = useState<{ old?: unknown; new?: unknown } | null>(null);

  const load = useCallback(async () => {
    if (!sessionId || !path || !hasRoots) return;
    setLoading(true);
    setFetchErr(null);
    try {
      const data = await api.sessionContent(sessionId, path, mode, contentOffset);
      setPayload({ old: data.old, new: data.new });
    } catch (e) {
      setFetchErr(String(e));
      setPayload(null);
    } finally {
      setLoading(false);
    }
  }, [sessionId, path, mode, hasRoots, contentOffset]);

  useLayoutEffect(() => {
    setContentOffset(0);
  }, [path, mode]);

  useEffect(() => {
    if (!open || !path) return;
    void load();
  }, [open, path, mode, load]);

  useEffect(() => {
    if (!open) {
      setFetchErr(null);
      setPayload(null);
    }
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open || !path) return null;

  const oldT = payload ? readSideText(payload.old) : "";
  const newT = payload ? readSideText(payload.new) : "";
  const oldB = payload ? readSideB64(payload.old) : new Uint8Array(0);
  const newB = payload ? readSideB64(payload.new) : new Uint8Array(0);
  const errOld = payload ? readSideErr(payload.old) : null;
  const errNew = payload ? readSideErr(payload.new) : null;
  const oldFileSize = payload ? readSideFileSize(payload.old) : undefined;
  const newFileSize = payload ? readSideFileSize(payload.new) : undefined;
  const offset =
    payload &&
    typeof payload.old === "object" &&
    payload.old &&
    "offset" in payload.old &&
    typeof (payload.old as Record<string, unknown>).offset === "number"
      ? ((payload.old as Record<string, unknown>).offset as number)
      : 0;
  const tailOff =
    oldFileSize != null && newFileSize != null ? tailContentOffset(oldFileSize, newFileSize) : 0;
  const canJumpTail = tailOff > 0 && contentOffset !== tailOff;
  const sizeMismatchHint =
    !loading && !fetchErr && payload
      ? reportDiskMismatchMessage(reportEntry ?? null, oldFileSize, newFileSize)
      : null;

  return (
    <div
      className="content-modal-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        className="content-modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="content-modal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <header className="content-modal-head">
          <div className="content-modal-title-wrap">
            <h2 id="content-modal-title" className="content-modal-title">
              Сравнение фрагмента
            </h2>
            <div className="content-modal-path" title={path}>
              {path}
            </div>
            {reportEntry && !reportEntry.is_dir && (
              <div className="content-modal-report-line">
                В отчёте:{" "}
                <strong>
                  {reportEntry.old_str}
                  {!reportEntry.unknown_old ? ` (${reportEntry.old_size} Б)` : ""}
                </strong>
                {" → "}
                <strong>
                  {reportEntry.new_str} ({reportEntry.new_size} Б)
                </strong>
              </div>
            )}
          </div>
          <div className="content-modal-actions">
            <div className="row content-mode-row">
              <button
                type="button"
                className={mode === "text" ? "" : "secondary"}
                onClick={() => setMode("text")}
              >
                Text
              </button>
              <button type="button" className={mode === "hex" ? "" : "secondary"} onClick={() => setMode("hex")}>
                Hex
              </button>
            </div>
            <label className="content-hide-unchanged">
              <input
                type="checkbox"
                checked={hideUnchanged}
                onChange={(e) => setHideUnchanged(e.target.checked)}
              />
              Скрыть неизменённые
            </label>
            {mode === "hex" &&
              oldFileSize != null &&
              newFileSize != null &&
              tailOff > 0 &&
              (canJumpTail ? (
                <button
                  type="button"
                  className="secondary content-jump-tail"
                  title={`Загрузить последние ${(CONTENT_SLICE_LIMIT / 1024).toFixed(0)} KiB (offset ${tailOff})`}
                  onClick={() => setContentOffset(tailOff)}
                >
                  К концу файла
                </button>
              ) : (
                <button type="button" className="secondary content-jump-tail" onClick={() => setContentOffset(0)}>
                  С начала файла
                </button>
              ))}
            <button type="button" className="secondary content-modal-close" onClick={onClose}>
              Закрыть
            </button>
          </div>
        </header>

        {!hasRoots ? (
          <p className="content-modal-err">В метаданных нет пары корней снимков — сравнение недоступно.</p>
        ) : loading ? (
          <p className="content-modal-loading">Загрузка…</p>
        ) : fetchErr ? (
          <p className="content-modal-err">{fetchErr}</p>
        ) : (
          <div className="content-modal-body">
            {(errOld || errNew) && (
              <div className="content-side-errs">
                {errOld && (
                  <p>
                    <strong>Старый:</strong> {errOld}
                  </p>
                )}
                {errNew && (
                  <p>
                    <strong>Новый:</strong> {errNew}
                  </p>
                )}
              </div>
            )}
            {sizeMismatchHint && (
              <div className="content-report-mismatch" role="status">
                <strong>Сверка с отчётом:</strong> {sizeMismatchHint}
              </div>
            )}
            {mode === "text" ? (
              <TextDiffViewerPane oldStr={oldT} newStr={newT} showDiffOnly={hideUnchanged} />
            ) : (
              <HexSplitView
                oldBuf={oldB}
                newBuf={newB}
                baseOffset={offset}
                hideEqualRows={hideUnchanged}
                oldFileSize={oldFileSize}
                newFileSize={newFileSize}
              />
            )}
            <p className="content-modal-foot">
              Фрагмент на экране: смещение <strong>{offset}</strong> (байт), до{" "}
              <strong>{CONTENT_SLICE_LIMIT / 1024}</strong> KiB на запрос. Для длинных файлов отличие может быть только
              ближе к концу — для hex есть кнопка «К концу файла».
              {hideUnchanged && mode === "text" && (
                <>
                  {" "}
                  Свёрнутый участок слева и справа можно развернуть кликом по строке с «@@».
                </>
              )}
              {hideUnchanged && mode === "hex" && <> Показаны только строки, где есть отличающийся байт.</>}
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
