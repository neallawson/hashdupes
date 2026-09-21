<script lang="ts">
  import {
    Folder,
    Files,
    HardDrive,
    Search,
    ShieldCheck,
    ShieldX,
    ShieldAlert,
    Split,
    X,
  } from "lucide-svelte";
  import { cn } from "$lib/utils";
  import { formatBytes, formatCount, pluralize } from "$lib/format";
  import Badge from "$lib/components/ui/Badge.svelte";
  import Button from "$lib/components/ui/Button.svelte";
  import Input from "$lib/components/ui/Input.svelte";
  import Progress from "$lib/components/ui/Progress.svelte";
  import Select from "$lib/components/ui/Select.svelte";
  import SwitchField from "$lib/components/ui/SwitchField.svelte";
  import FileGroup from "$lib/components/FileGroup.svelte";
  import FolderGroup from "$lib/components/FolderGroup.svelte";
  import { Verification } from "$lib/verification.svelte";
  import type { FileGroupDTO, FolderGroupDTO, ReportDTO } from "$lib/types";

  let { report }: { report: ReportDTO } = $props();

  type Tab = "folders" | "files";
  type SortKey = "reclaimable" | "size" | "copies" | "name" | "path";

  const sortOptions = [
    { value: "reclaimable" as SortKey, label: "Most reclaimable" },
    { value: "size" as SortKey, label: "Largest" },
    { value: "copies" as SortKey, label: "Most copies" },
    { value: "name" as SortKey, label: "Name (A→Z)" },
    { value: "path" as SortKey, label: "Path (A→Z)" },
  ];

  /** Groups rendered per page; expanded rows make true windowing impractical. */
  const PAGE = 50;

  // One verification instance per report: it is discarded when a new scan
  // replaces the report, which is the correct lifetime for these results.
  const verification = new Verification();

  let tab = $state<Tab>("folders");
  let hideSubsumed = $state(true);
  let filter = $state("");
  let sort = $state<SortKey>("reclaimable");
  let limit = $state(PAGE);

  const needle = $derived(filter.trim().toLowerCase());

  const subsumedCount = $derived(
    report.fileGroups.filter((g) => g.withinDupFolder).length,
  );

  function matchesFile(g: FileGroupDTO): boolean {
    if (!needle) return true;
    return g.files.some((f) => f.path.toLowerCase().includes(needle));
  }

  function matchesFolder(g: FolderGroupDTO): boolean {
    if (!needle) return true;
    return g.folders.some((f) => f.path.toLowerCase().includes(needle));
  }

  function sortFiles(groups: FileGroupDTO[]): FileGroupDTO[] {
    const out = [...groups];
    switch (sort) {
      case "reclaimable":
        // Deliberately the candidate figure, not the verified one: ordering has
        // to stay put while a batch verify updates results underneath it.
        return out.sort((a, b) => b.reclaimable - a.reclaimable);
      case "size":
        return out.sort((a, b) => b.size - a.size);
      case "copies":
        return out.sort((a, b) => b.files.length - a.files.length);
      case "name":
        return out.sort((a, b) =>
          (a.files[0]?.name ?? "").localeCompare(b.files[0]?.name ?? ""),
        );
      case "path":
        return out.sort((a, b) =>
          (a.files[0]?.path ?? "").localeCompare(b.files[0]?.path ?? ""),
        );
    }
  }

  function sortFolders(groups: FolderGroupDTO[]): FolderGroupDTO[] {
    const out = [...groups];
    switch (sort) {
      case "reclaimable":
        return out.sort((a, b) => b.reclaimable - a.reclaimable);
      case "size":
        return out.sort(
          (a, b) => (b.folders[0]?.totalSize ?? 0) - (a.folders[0]?.totalSize ?? 0),
        );
      case "copies":
        return out.sort((a, b) => b.folders.length - a.folders.length);
      case "name":
        return out.sort((a, b) =>
          (a.folders[0]?.name ?? "").localeCompare(b.folders[0]?.name ?? ""),
        );
      case "path":
        return out.sort((a, b) =>
          (a.folders[0]?.path ?? "").localeCompare(b.folders[0]?.path ?? ""),
        );
    }
  }

  const visibleFileGroups = $derived(
    sortFiles(
      report.fileGroups.filter(
        (g) => (!hideSubsumed || !g.withinDupFolder) && matchesFile(g),
      ),
    ),
  );
  const visibleFolderGroups = $derived(
    sortFolders(report.folderGroups.filter(matchesFolder)),
  );

  // Reclaimable bytes track confirmed results once a group has been verified,
  // so these figures correct themselves as verification proceeds.
  const folderReclaimable = $derived(
    report.folderGroups.reduce((sum, g) => sum + g.reclaimable, 0),
  );
  // Sets inside a duplicate folder are already counted in that folder's figure,
  // so they are excluded here to keep the total from double-counting.
  const fileReclaimable = $derived(
    report.fileGroups
      .filter((g) => !g.withinDupFolder)
      .reduce((sum, g) => sum + verification.reclaimableFor(g), 0),
  );

  // Set count tracks confirmed reality too, so a rejected candidate stops being
  // counted and a split one counts once per confirmed set.
  const fileSetCount = $derived(
    report.fileGroups.reduce((n, g) => n + verification.setCountFor(g), 0),
  );

  const tally = $derived(verification.tally);
  const settled = $derived(verification.settledCount);
  const pendingShown = $derived(
    visibleFileGroups.filter(
      (g) => verification.result(g.id).outcome === "unverified",
    ).length,
  );

  const renderedFileGroups = $derived(visibleFileGroups.slice(0, limit));
  const renderedFolderGroups = $derived(visibleFolderGroups.slice(0, limit));
  const visibleCount = $derived(
    tab === "files" ? visibleFileGroups.length : visibleFolderGroups.length,
  );

  // Default to the tab that actually has content.
  $effect(() => {
    if (report.folderGroups.length === 0 && report.fileGroups.length > 0) {
      tab = "files";
    }
  });

  // Start a fresh page whenever the visible set changes shape.
  $effect(() => {
    void [tab, needle, sort, hideSubsumed];
    limit = PAGE;
  });

  function selectTab(next: Tab) {
    tab = next;
  }
</script>

<div class="flex flex-col gap-4">
  <!-- Summary cards -->
  <div class="grid grid-cols-3 gap-3">
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Folder class="size-4" /> Duplicate folders
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {formatCount(report.folderGroups.length)}
      </div>
      <div class="text-xs text-muted-foreground">
        {formatBytes(folderReclaimable)} reclaimable
      </div>
    </div>
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Files class="size-4" /> Duplicate file sets
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {formatCount(fileSetCount)}
      </div>
      <div class="text-xs text-muted-foreground">
        {formatBytes(fileReclaimable)} reclaimable
        {#if subsumedCount > 0}
          · excludes {formatCount(subsumedCount)} inside dup folders
        {/if}
      </div>
    </div>
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <HardDrive class="size-4" /> Total reclaimable
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {formatBytes(fileReclaimable + folderReclaimable)}
      </div>
      <div class="text-xs text-muted-foreground">
        {#if report.fileGroups.length === 0}
          if one copy of each is kept
        {:else if settled >= report.fileGroups.length}
          confirmed by byte-compare
        {:else}
          estimate · {formatCount(settled)} of {formatCount(
            report.fileGroups.length,
          )} sets confirmed
        {/if}
      </div>
    </div>
  </div>

  <!-- Verification bar: the instrument for checking how many candidate groups
       survive the byte-compare. -->
  {#if report.fileGroups.length > 0}
    <div class="rounded-lg border bg-card px-4 py-3">
      <div class="flex flex-wrap items-center gap-3">
        <span class="text-sm font-medium">Verification</span>

        {#if settled === 0 && !verification.batchRunning}
          <span class="text-xs text-muted-foreground">
            Nothing compared yet — expand a set, or verify them all at once.
          </span>
        {:else}
          <div class="flex flex-wrap items-center gap-1.5">
            {#if tally.confirmed > 0}
              <Badge variant="success" class="gap-1">
                <ShieldCheck class="size-3" /> {formatCount(tally.confirmed)} confirmed
              </Badge>
            {/if}
            {#if tally.split > 0}
              <Badge
                variant="outline"
                class="gap-1 border-amber-500/40 text-amber-500"
              >
                <Split class="size-3" /> {formatCount(tally.split)} split
              </Badge>
            {/if}
            {#if tally.rejected > 0}
              <Badge variant="destructive" class="gap-1">
                <ShieldX class="size-3" /> {formatCount(tally.rejected)} rejected
              </Badge>
            {/if}
            {#if tally.failed > 0}
              <Badge variant="destructive" class="gap-1">
                <ShieldAlert class="size-3" /> {formatCount(tally.failed)} failed
              </Badge>
            {/if}
          </div>
        {/if}

        <div class="ml-auto flex items-center gap-2">
          {#if verification.batchRunning}
            <span class="text-xs tabular-nums text-muted-foreground">
              {formatCount(verification.batchDone)} / {formatCount(
                verification.batchTotal,
              )}
            </span>
            <Button variant="outline" size="sm" onclick={() => verification.stop()}>
              <X class="size-3.5" /> Stop
            </Button>
          {:else}
            <Button
              variant="outline"
              size="sm"
              disabled={pendingShown === 0}
              onclick={() => verification.verifyAll(visibleFileGroups)}
            >
              <ShieldCheck class="size-3.5" />
              {pendingShown === 0
                ? "All shown sets verified"
                : `Verify ${formatCount(pendingShown)} shown`}
            </Button>
          {/if}
        </div>
      </div>

      {#if verification.batchRunning}
        <Progress
          class="mt-2"
          value={verification.batchDone}
          max={verification.batchTotal}
        />
      {/if}

      {#if tally.rejected > 0}
        <p class="mt-2 text-xs text-muted-foreground">
          Rejected sets shared a size and content header but differed on compare —
          expected, and the reason a byte-compare gates every result.
        </p>
      {/if}
    </div>
  {/if}

  <!-- Tabs + controls -->
  <div class="flex flex-wrap items-end justify-between gap-3 border-b">
    <div class="flex gap-1">
      <button
        type="button"
        onclick={() => selectTab("folders")}
        class={cn(
          "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
          tab === "folders"
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground",
        )}
      >
        Folders ({formatCount(visibleFolderGroups.length)}{report.folderGroups
          .length !== visibleFolderGroups.length
          ? ` of ${formatCount(report.folderGroups.length)}`
          : ""})
      </button>
      <button
        type="button"
        onclick={() => selectTab("files")}
        class={cn(
          "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
          tab === "files"
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground",
        )}
      >
        Files ({formatCount(visibleFileGroups.length)}{report.fileGroups
          .length !== visibleFileGroups.length
          ? ` of ${formatCount(report.fileGroups.length)}`
          : ""})
      </button>
    </div>

    <div class="flex flex-wrap items-center gap-3 pb-2">
      {#if tab === "files" && subsumedCount > 0}
        <SwitchField
          label="Hide sets inside dup folders"
          labelClass="text-xs text-muted-foreground"
          checked={hideSubsumed}
          onCheckedChange={(v) => (hideSubsumed = v)}
        />
      {/if}
      <div class="relative">
        <Search
          class="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
        />
        <Input
          type="search"
          bind:value={filter}
          ariaLabel="Filter by path"
          placeholder="Filter by path…"
          class="w-56 pl-8"
        />
      </div>
      <Select
        bind:value={sort}
        options={sortOptions}
        ariaLabel="Sort order"
        class="w-44"
      />
    </div>
  </div>

  <!-- Lists -->
  {#if tab === "folders"}
    {#if visibleFolderGroups.length === 0}
      <p class="py-12 text-center text-sm text-muted-foreground">
        {report.folderGroups.length === 0
          ? "No duplicate folders found."
          : "No duplicate folders match this filter."}
      </p>
    {:else}
      <div class="flex flex-col gap-2">
        {#each renderedFolderGroups as g (g.id)}
          <FolderGroup group={g} />
        {/each}
      </div>
    {/if}
  {:else if visibleFileGroups.length === 0}
    <p class="py-12 text-center text-sm text-muted-foreground">
      {report.fileGroups.length === 0
        ? "No duplicate files found."
        : "No duplicate file sets match these filters."}
    </p>
  {:else}
    <div class="flex flex-col gap-2">
      {#each renderedFileGroups as g (g.id)}
        <FileGroup group={g} {verification} />
      {/each}
    </div>
  {/if}

  {#if visibleCount > limit}
    <div class="flex items-center justify-center gap-3 pb-4">
      <span class="text-xs text-muted-foreground">
        Showing {formatCount(limit)} of {formatCount(visibleCount)}
      </span>
      <Button variant="outline" size="sm" onclick={() => (limit += PAGE)}>
        Show {pluralize(Math.min(PAGE, visibleCount - limit), "more")}
      </Button>
    </div>
  {/if}
</div>
