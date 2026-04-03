export type Snapshot = { path: string; label: string };

export type SkippedPath = { path: string; reason: string };

export type ReportMeta = {
  kind?: string;
  v?: number;
  old_root: string;
  new_root: string;
  old_label?: string;
  new_label?: string;
  created_at?: string;
  options?: Record<string, unknown>;
  totals?: { changed_files: number; size_bytes: number };
  skipped?: SkippedPath[];
  skipped_total?: number;
  skipped_truncated?: boolean;
};

export type ReportEntry = {
  kind?: string;
  path: string;
  old_size: number;
  new_size: number;
  old_str: string;
  new_str: string;
  is_dir: boolean;
  is_symlink?: boolean;
  unknown_old: boolean;
  in_dir?: number;
};

export type ChildDTO = {
  path: string;
  name: string;
  is_dir: boolean;
  entry?: ReportEntry | null;
};
