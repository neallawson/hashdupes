import { api } from "$lib/api";
import type { FileGroupDTO } from "$lib/types";

/**
 * Outcome of byte-comparing a candidate group.
 *
 * - `unverified` — no compare has run yet; the candidate is unconfirmed.
 * - `verifying` — a compare is in flight.
 * - `confirmed` — every candidate member matched, as a single set.
 * - `split`     — members matched only in part, yielding smaller sets and/or
 *                 dropping members.
 * - `rejected`  — no two members matched; the candidate was a head-hash
 *                 collision.
 * - `failed`    — the compare could not complete (I/O or permission error).
 */
export type Outcome =
  | "unverified"
  | "verifying"
  | "confirmed"
  | "split"
  | "rejected"
  | "failed";

/** Verification state for one candidate group. */
export interface GroupResult {
  outcome: Outcome;
  /** Confirmed subgroups; empty unless the outcome is confirmed or split. */
  groups: FileGroupDTO[];
  /** Reclaimable bytes according to the confirmed subgroups. */
  reclaimable: number;
  /** Per-path errors reported by the compare, if any. */
  error: string | null;
}

const UNVERIFIED: GroupResult = Object.freeze({
  outcome: "unverified",
  groups: [],
  reclaimable: 0,
  error: null,
});

/** Tally of outcomes across a batch, for reporting pipeline health. */
export interface OutcomeTally {
  confirmed: number;
  split: number;
  rejected: number;
  failed: number;
}

/** How many compares to run concurrently during a batch verify. */
const BATCH_CONCURRENCY = 4;

/**
 * Verification holds the byte-compare results for a report's candidate groups.
 *
 * State lives here rather than inside each group component so that summary
 * totals can reflect confirmed rather than candidate figures, and so a batch
 * verify can drive every group at once. One instance belongs to one report.
 */
export class Verification {
  #results = $state<Record<string, GroupResult>>({});
  #batchTotal = $state(0);
  #batchDone = $state(0);
  #stopped = false;

  /** Result for a group id; never null, defaulting to unverified. */
  result(id: string): GroupResult {
    return this.#results[id] ?? UNVERIFIED;
  }

  /** Number of groups with a settled (non-pending) result. */
  get settledCount(): number {
    return Object.values(this.#results).filter(
      (r) => r.outcome !== "verifying" && r.outcome !== "unverified",
    ).length;
  }

  /** True while a batch verify is in progress. */
  get batchRunning(): boolean {
    return this.#batchTotal > 0 && this.#batchDone < this.#batchTotal;
  }

  get batchTotal(): number {
    return this.#batchTotal;
  }

  get batchDone(): number {
    return this.#batchDone;
  }

  /** Counts of each settled outcome, for a batch summary. */
  get tally(): OutcomeTally {
    const t: OutcomeTally = { confirmed: 0, split: 0, rejected: 0, failed: 0 };
    for (const r of Object.values(this.#results)) {
      if (r.outcome in t) t[r.outcome as keyof OutcomeTally]++;
    }
    return t;
  }

  /**
   * Reclaimable bytes for a group: the confirmed figure once verified, falling
   * back to the candidate's own estimate while it is still unverified.
   */
  reclaimableFor(group: FileGroupDTO): number {
    const r = this.result(group.id);
    switch (r.outcome) {
      case "unverified":
      case "verifying":
      case "failed":
        return group.reclaimable;
      default:
        return r.reclaimable;
    }
  }

  /**
   * Number of duplicate sets a candidate actually represents: zero once it has
   * been rejected, more than one once it has split, and otherwise itself.
   */
  setCountFor(group: FileGroupDTO): number {
    const r = this.result(group.id);
    switch (r.outcome) {
      case "confirmed":
      case "split":
        return r.groups.length;
      case "rejected":
        return 0;
      default:
        return 1;
    }
  }

  /**
   * verify byte-compares one candidate group and records the outcome. Groups
   * already verified or in flight are left alone, so repeated expansion of a
   * row costs nothing.
   */
  async verify(group: FileGroupDTO): Promise<void> {
    const existing = this.result(group.id);
    if (existing.outcome !== "unverified") return;

    this.#results[group.id] = { ...UNVERIFIED, outcome: "verifying" };
    try {
      const res = await api.verifyGroup({
        size: group.size,
        paths: group.files.map((f) => f.path),
      });
      const groups = res.groups ?? [];
      const errors = res.errors ?? {};
      const errorText = Object.entries(errors)
        .map(([path, msg]) => `${path}: ${msg}`)
        .join("; ");

      const confirmedMembers = groups.reduce((n, g) => n + g.files.length, 0);
      let outcome: Outcome;
      if (groups.length === 0) {
        outcome = "rejected";
      } else if (
        groups.length === 1 &&
        confirmedMembers === group.files.length
      ) {
        outcome = "confirmed";
      } else {
        outcome = "split";
      }

      this.#results[group.id] = {
        outcome,
        groups,
        reclaimable: groups.reduce((sum, g) => sum + g.reclaimable, 0),
        error: errorText || null,
      };
    } catch (e) {
      this.#results[group.id] = {
        ...UNVERIFIED,
        outcome: "failed",
        error: e instanceof Error ? e.message : String(e),
      };
    }
  }

  /**
   * verifyAll compares every not-yet-verified group with bounded concurrency,
   * so the whole candidate set can be checked in one pass. Byte-compares are
   * I/O bound, hence the small worker count.
   */
  async verifyAll(groups: FileGroupDTO[]): Promise<void> {
    const pending = groups.filter(
      (g) => this.result(g.id).outcome === "unverified",
    );
    if (pending.length === 0) return;

    this.#stopped = false;
    this.#batchTotal = pending.length;
    this.#batchDone = 0;

    let next = 0;
    const worker = async () => {
      while (!this.#stopped) {
        const i = next++;
        if (i >= pending.length) return;
        await this.verify(pending[i]);
        this.#batchDone++;
      }
    };
    await Promise.all(
      Array.from(
        { length: Math.min(BATCH_CONCURRENCY, pending.length) },
        worker,
      ),
    );
    // A stopped batch leaves the counter short; square it up so batchRunning
    // reports false.
    this.#batchDone = this.#batchTotal;
  }

  /** stop halts an in-flight batch after the current compares settle. */
  stop(): void {
    this.#stopped = true;
  }
}
