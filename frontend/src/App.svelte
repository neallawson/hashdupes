<script lang="ts">
  import { onMount } from "svelte";
  import {
    FolderSearch,
    Play,
    X,
    Loader2,
    AlertTriangle,
    Copy,
    SearchX,
    CheckCircle2,
    Ban,
  } from "lucide-svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import Input from "$lib/components/ui/Input.svelte";
  import Select from "$lib/components/ui/Select.svelte";
  import SwitchField from "$lib/components/ui/SwitchField.svelte";
  import Progress from "$lib/components/ui/Progress.svelte";
  import Results from "$lib/components/Results.svelte";
  import { api, onEvent, isWailsAvailable } from "$lib/api";
  import {
    formatBytes,
    formatCount,
    formatDuration,
    formatRate,
    pluralize,
  } from "$lib/format";
  import type {
    HashAlgo,
    ReportDTO,
    ScanDone,
    ScanError,
    ScanProgress,
  } from "$lib/types";

  type Status = "idle" | "scanning" | "done" | "error";

  const algoOptions = [
    { value: "blake3" as HashAlgo, label: "BLAKE3" },
    { value: "sha256" as HashAlgo, label: "SHA-256" },
  ];

  let wailsReady = $state(false);
  let status = $state<Status>("idle");
  let rootPath = $state("");
  let algo = $state<HashAlgo>("blake3");
  let ignoreHidden = $state(true);
  let followSymlinks = $state(false);

  let progress = $state<ScanProgress | null>(null);
  let summary = $state<ScanDone | null>(null);
  let report = $state<ReportDTO | null>(null);
  let errorMsg = $state<string | null>(null);

  // Wall-clock tracking so throughput and elapsed time are visible during a
  // scan; both are derived here rather than reported by the backend.
  let startedAt = $state<number | null>(null);
  let now = $state(Date.now());
  let elapsedMs = $state(0);

  const canScan = $derived(rootPath.trim().length > 0 && status !== "scanning");
  const liveElapsedMs = $derived(startedAt === null ? 0 : now - startedAt);
  const byteRate = $derived(
    progress && liveElapsedMs > 500
      ? (progress.bytesHashed / liveElapsedMs) * 1000
      : 0,
  );
  const fileRate = $derived(
    progress && liveElapsedMs > 500
      ? (progress.filesHashed / liveElapsedMs) * 1000
      : 0,
  );

  onMount(() => {
    wailsReady = isWailsAvailable();
    const offProgress = onEvent("scan:progress", (p: ScanProgress) => {
      progress = p;
    });
    const offDone = onEvent("scan:done", async (d: ScanDone) => {
      progress = { ...(progress ?? blankProgress()), done: true };
      summary = d;
      elapsedMs = startedAt === null ? 0 : Date.now() - startedAt;
      await loadReport(d.scanId);
      if (status !== "error") status = "done";
    });
    const offError = onEvent("scan:error", (e: ScanError) => {
      errorMsg = e.message;
      elapsedMs = startedAt === null ? 0 : Date.now() - startedAt;
      status = "error";
    });
    return () => {
      offProgress();
      offDone();
      offError();
    };
  });

  // Tick only while a scan is running so elapsed time and rates advance.
  $effect(() => {
    if (status !== "scanning") return;
    const t = setInterval(() => (now = Date.now()), 250);
    return () => clearInterval(t);
  });

  function blankProgress(): ScanProgress {
    return {
      filesSeen: 0,
      filesHashed: 0,
      bytesHashed: 0,
      skipped: 0,
      currentPath: "",
      done: false,
    };
  }

  async function chooseFolder() {
    try {
      const path = await api.chooseFolder();
      if (path) rootPath = path;
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : String(e);
    }
  }

  async function startScan() {
    if (!canScan) return;
    errorMsg = null;
    report = null;
    summary = null;
    progress = blankProgress();
    startedAt = Date.now();
    now = startedAt;
    elapsedMs = 0;
    status = "scanning";
    try {
      await api.startScan({ rootPath, algo, ignoreHidden, followSymlinks });
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : String(e);
      status = "error";
    }
  }

  async function cancelScan() {
    try {
      await api.cancelScan();
    } catch {
      /* ignore */
    }
  }

  async function loadReport(scanId: number) {
    try {
      report = await api.getReport(scanId);
    } catch (e) {
      errorMsg = e instanceof Error ? e.message : String(e);
      status = "error";
    }
  }
</script>

<div class="flex h-full flex-col">
  <!-- Header -->
  <header class="flex items-center gap-3 border-b px-6 py-3">
    <Copy class="size-5 text-primary" />
    <h1 class="text-lg font-semibold tracking-tight">hashdupes</h1>
    <span
      class="whitespace-nowrap rounded bg-secondary px-1.5 py-0.5 text-[10px] font-medium uppercase text-muted-foreground"
    >
      read-only
    </span>
    <div class="ml-auto text-xs text-muted-foreground">
      {#if status === "scanning"}Scanning…{:else if status === "done"}Scan complete{/if}
    </div>
  </header>

  <!-- Controls -->
  <section class="border-b px-6 py-4">
    <div class="flex flex-wrap items-end gap-3">
      <div class="flex min-w-0 flex-1 flex-col gap-1">
        <label for="root" class="text-xs font-medium text-muted-foreground">
          Folder to scan
        </label>
        <div class="flex gap-2">
          <Input
            id="root"
            bind:value={rootPath}
            placeholder="/path/to/scan"
            class="flex-1"
          />
          <Button variant="outline" onclick={chooseFolder} disabled={!wailsReady}>
            <FolderSearch class="size-4" /> Browse
          </Button>
        </div>
      </div>

      <div class="flex flex-col gap-1">
        <label for="algo" class="text-xs font-medium text-muted-foreground">
          Algorithm
        </label>
        <Select id="algo" bind:value={algo} options={algoOptions} class="w-32" />
      </div>

      <div class="flex items-center gap-4 pb-2">
        <SwitchField
          label="Ignore hidden"
          checked={ignoreHidden}
          onCheckedChange={(v) => (ignoreHidden = v)}
        />
        <SwitchField
          label="Follow symlinks"
          checked={followSymlinks}
          onCheckedChange={(v) => (followSymlinks = v)}
        />
      </div>

      <div class="flex gap-2 pb-0.5">
        {#if status === "scanning"}
          <Button variant="destructive" onclick={cancelScan}>
            <X class="size-4" /> Cancel
          </Button>
        {:else}
          <Button onclick={startScan} disabled={!canScan}>
            <Play class="size-4" /> Scan
          </Button>
        {/if}
      </div>
    </div>

    {#if !wailsReady}
      <p class="mt-3 flex items-center gap-2 text-xs text-amber-500">
        <AlertTriangle class="size-3.5" />
        Desktop bindings not detected — run via
        <code class="rounded bg-secondary px-1">wails dev</code> or a built binary.
      </p>
    {/if}
  </section>

  <!-- Live progress -->
  {#if status === "scanning" && progress}
    <section class="border-b px-6 py-3">
      <div class="mb-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
        <Loader2 class="size-4 animate-spin text-primary" />
        <span class="font-medium">
          {pluralize(progress.filesHashed, "file")} hashed
        </span>
        <span class="text-muted-foreground">
          · {formatCount(progress.filesSeen)} seen
          · {formatBytes(progress.bytesHashed)} read
          {#if progress.skipped > 0}
            · <span class="text-amber-500"
              >{formatCount(progress.skipped)} skipped</span
            >
          {/if}
        </span>
        <span class="ml-auto tabular-nums text-muted-foreground">
          {formatDuration(liveElapsedMs)}
          · {formatRate(byteRate)}
          · {fileRate > 0 ? `${Math.round(fileRate).toLocaleString()} files/s` : "—"}
        </span>
      </div>
      <Progress indeterminate={true} />
      {#if progress.currentPath}
        <p class="mt-2 truncate text-xs text-muted-foreground">
          {progress.currentPath}
        </p>
      {/if}
    </section>
  {/if}

  <!-- Completed-scan summary: the counts the pipeline actually reported. -->
  {#if status === "done" && summary}
    <section
      class="flex flex-wrap items-center gap-x-4 gap-y-1 border-b px-6 py-2 text-xs"
    >
      <span class="flex items-center gap-1.5 font-medium text-emerald-500">
        <CheckCircle2 class="size-3.5" /> Scan #{summary.scanId}
      </span>
      <span class="text-muted-foreground">
        {formatCount(summary.filesHashed)} files hashed
        · {formatCount(summary.foldersSeen)} folders
        · finished in {formatDuration(elapsedMs)}
      </span>
      {#if summary.skipped > 0}
        <span class="flex items-center gap-1.5 text-amber-500">
          <Ban class="size-3.5" />
          {pluralize(summary.skipped, "entry", "entries")} skipped (unreadable,
          symlinked, or filtered)
        </span>
      {/if}
    </section>
  {/if}

  <!-- Results / states -->
  <main class="scrollbar-thin flex-1 overflow-y-auto px-6 py-5">
    {#if status === "error"}
      <div
        class="mx-auto mt-12 max-w-md rounded-lg border border-destructive/40 bg-destructive/10 p-4 text-center"
      >
        <AlertTriangle class="mx-auto size-6 text-destructive" />
        <p class="mt-2 font-medium">Scan failed</p>
        <p class="mt-1 text-sm text-muted-foreground">{errorMsg}</p>
      </div>
    {:else if status === "done" && report}
      {#if report.folderGroups.length === 0 && report.fileGroups.length === 0}
        <div class="mt-16 text-center text-muted-foreground">
          <SearchX class="mx-auto size-8" />
          <p class="mt-2 font-medium text-foreground">No duplicates found</p>
          <p class="text-sm">
            Nothing in this folder matched on size and content header.
          </p>
        </div>
      {:else}
        <Results {report} />
      {/if}
    {:else if status === "idle"}
      <div class="mt-16 text-center text-muted-foreground">
        <FolderSearch class="mx-auto size-8" />
        <p class="mt-2 font-medium text-foreground">
          Choose a folder and start a scan
        </p>
        <p class="text-sm">
          hashdupes finds duplicate files and folders without modifying anything.
        </p>
      </div>
    {/if}
  </main>
</div>
