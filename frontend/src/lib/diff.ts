// Tiny line-level unified diff for the prompt history viewer.
//
// Implementation: longest common subsequence (LCS) over lines, then walk the
// LCS table back-to-front to emit "context" / "added" / "deleted" rows. We
// only ever diff one prompt against another (a few hundred lines at most), so
// the O(N*M) table is fine; we don't need patience or Myers here.

export type DiffOp = "ctx" | "add" | "del";

export interface DiffLine {
  op: DiffOp;
  oldNo: number | null; // 1-based line number in the "old" version, null for adds
  newNo: number | null; // 1-based line number in the "new" version, null for dels
  text: string;
}

export interface DiffHunk {
  oldStart: number;
  newStart: number;
  lines: DiffLine[];
}

export interface DiffResult {
  add: number;
  del: number;
  hunks: DiffHunk[];
}

/**
 * Compute a unified diff between `oldText` and `newText` at line granularity.
 * Returns hunks of contiguous changed sections plus a small amount of
 * surrounding context (default 3 lines), and totals of additions / deletions.
 */
export function diffLines(
  oldText: string,
  newText: string,
  contextLines = 3,
): DiffResult {
  const oldLines = splitLines(oldText);
  const newLines = splitLines(newText);
  const ops = lcsOps(oldLines, newLines);

  let oldNo = 0;
  let newNo = 0;
  const annotated: DiffLine[] = ops.map((op) => {
    switch (op.kind) {
      case "ctx":
        oldNo += 1;
        newNo += 1;
        return { op: "ctx", oldNo, newNo, text: oldLines[op.oldIdx] };
      case "del":
        oldNo += 1;
        return { op: "del", oldNo, newNo: null, text: oldLines[op.oldIdx] };
      case "add":
        newNo += 1;
        return { op: "add", oldNo: null, newNo, text: newLines[op.newIdx] };
    }
  });

  let add = 0;
  let del = 0;
  for (const line of annotated) {
    if (line.op === "add") add += 1;
    else if (line.op === "del") del += 1;
  }

  const hunks = groupIntoHunks(annotated, contextLines);
  return { add, del, hunks };
}

function splitLines(text: string): string[] {
  if (text === "") return [];
  // Preserve the absence of a trailing newline by not adding an empty entry
  // when the input ends with one. This matches typical git/diff behavior.
  const parts = text.split("\n");
  if (parts[parts.length - 1] === "") parts.pop();
  return parts;
}

type Op =
  | { kind: "ctx"; oldIdx: number; newIdx: number }
  | { kind: "del"; oldIdx: number }
  | { kind: "add"; newIdx: number };

function lcsOps(oldLines: string[], newLines: string[]): Op[] {
  const n = oldLines.length;
  const m = newLines.length;
  // dp[i][j] = LCS length of oldLines[0..i) and newLines[0..j).
  const dp: Uint32Array[] = Array.from({ length: n + 1 }, () => new Uint32Array(m + 1));
  for (let i = 1; i <= n; i += 1) {
    for (let j = 1; j <= m; j += 1) {
      if (oldLines[i - 1] === newLines[j - 1]) {
        dp[i][j] = dp[i - 1][j - 1] + 1;
      } else {
        dp[i][j] = Math.max(dp[i - 1][j], dp[i][j - 1]);
      }
    }
  }
  const ops: Op[] = [];
  let i = n;
  let j = m;
  while (i > 0 && j > 0) {
    if (oldLines[i - 1] === newLines[j - 1]) {
      ops.push({ kind: "ctx", oldIdx: i - 1, newIdx: j - 1 });
      i -= 1;
      j -= 1;
    } else if (dp[i - 1][j] >= dp[i][j - 1]) {
      ops.push({ kind: "del", oldIdx: i - 1 });
      i -= 1;
    } else {
      ops.push({ kind: "add", newIdx: j - 1 });
      j -= 1;
    }
  }
  while (i > 0) {
    ops.push({ kind: "del", oldIdx: i - 1 });
    i -= 1;
  }
  while (j > 0) {
    ops.push({ kind: "add", newIdx: j - 1 });
    j -= 1;
  }
  ops.reverse();
  return ops;
}

function groupIntoHunks(lines: DiffLine[], contextLines: number): DiffHunk[] {
  if (lines.length === 0) return [];
  // Identify indices that are part of a change-or-context window.
  const keep = new Array<boolean>(lines.length).fill(false);
  for (let i = 0; i < lines.length; i += 1) {
    if (lines[i].op !== "ctx") {
      const from = Math.max(0, i - contextLines);
      const to = Math.min(lines.length - 1, i + contextLines);
      for (let k = from; k <= to; k += 1) keep[k] = true;
    }
  }
  const hunks: DiffHunk[] = [];
  let cur: DiffLine[] = [];
  let curOldStart = 0;
  let curNewStart = 0;
  for (let i = 0; i < lines.length; i += 1) {
    if (!keep[i]) {
      if (cur.length > 0) {
        hunks.push({ oldStart: curOldStart, newStart: curNewStart, lines: cur });
        cur = [];
      }
      continue;
    }
    if (cur.length === 0) {
      curOldStart = lines[i].oldNo ?? 0;
      curNewStart = lines[i].newNo ?? 0;
    }
    cur.push(lines[i]);
  }
  if (cur.length > 0) {
    hunks.push({ oldStart: curOldStart, newStart: curNewStart, lines: cur });
  }
  return hunks;
}
