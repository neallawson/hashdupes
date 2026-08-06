/** formatBytes renders a byte count as a human-readable size (binary units). */
export function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(
    units.length - 1,
    Math.floor(Math.log(bytes) / Math.log(1024)),
  );
  const value = bytes / Math.pow(1024, i);
  const digits = value >= 100 || i === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[i]}`;
}

/** formatDate renders an RFC3339 timestamp as a compact local date-time. */
export function formatDate(rfc3339: string): string {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return rfc3339;
  return d.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** dirOf returns the parent directory portion of a path. */
export function dirOf(path: string): string {
  const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  return i <= 0 ? path : path.slice(0, i);
}

/** pluralize returns a count with a singular/plural noun. */
export function pluralize(n: number, singular: string, plural?: string): string {
  return `${n.toLocaleString()} ${n === 1 ? singular : (plural ?? singular + "s")}`;
}
