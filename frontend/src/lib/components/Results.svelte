<script lang="ts">
  import { Folder, Files, HardDrive } from "lucide-svelte";
  import { cn } from "$lib/utils";
  import { formatBytes, pluralize } from "$lib/format";
  import Switch from "$lib/components/ui/Switch.svelte";
  import FileGroup from "$lib/components/FileGroup.svelte";
  import FolderGroup from "$lib/components/FolderGroup.svelte";
  import type { ReportDTO } from "$lib/types";

  let { report }: { report: ReportDTO } = $props();

  type Tab = "folders" | "files";
  let tab = $state<Tab>("folders");
  let hideSubsumed = $state(true);

  const visibleFileGroups = $derived(
    hideSubsumed
      ? report.fileGroups.filter((g) => !g.withinDupFolder)
      : report.fileGroups,
  );
  const subsumedCount = $derived(
    report.fileGroups.filter((g) => g.withinDupFolder).length,
  );

  const fileReclaimable = $derived(
    report.fileGroups.reduce((sum, g) => sum + g.reclaimable, 0),
  );
  const folderReclaimable = $derived(
    report.folderGroups.reduce((sum, g) => sum + g.reclaimable, 0),
  );

  // Default to the tab that actually has content.
  $effect(() => {
    if (report.folderGroups.length === 0 && report.fileGroups.length > 0) {
      tab = "files";
    }
  });
</script>

<div class="flex flex-col gap-4">
  <!-- Summary cards -->
  <div class="grid grid-cols-3 gap-3">
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Folder class="size-4" /> Duplicate folders
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {report.folderGroups.length.toLocaleString()}
      </div>
      <div class="text-xs text-muted-foreground">{formatBytes(folderReclaimable)} reclaimable</div>
    </div>
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <Files class="size-4" /> Duplicate file sets
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {report.fileGroups.length.toLocaleString()}
      </div>
      <div class="text-xs text-muted-foreground">{formatBytes(fileReclaimable)} reclaimable</div>
    </div>
    <div class="rounded-lg border bg-card p-4">
      <div class="flex items-center gap-2 text-sm text-muted-foreground">
        <HardDrive class="size-4" /> Total reclaimable
      </div>
      <div class="mt-1 text-2xl font-semibold tabular-nums">
        {formatBytes(fileReclaimable + folderReclaimable)}
      </div>
      <div class="text-xs text-muted-foreground">if one copy of each is kept</div>
    </div>
  </div>

  <!-- Tabs -->
  <div class="flex items-center justify-between border-b">
    <div class="flex gap-1">
      <button
        type="button"
        onclick={() => (tab = "folders")}
        class={cn(
          "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
          tab === "folders"
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground",
        )}
      >
        Folders ({report.folderGroups.length})
      </button>
      <button
        type="button"
        onclick={() => (tab = "files")}
        class={cn(
          "border-b-2 px-4 py-2 text-sm font-medium transition-colors",
          tab === "files"
            ? "border-primary text-foreground"
            : "border-transparent text-muted-foreground hover:text-foreground",
        )}
      >
        Files ({visibleFileGroups.length})
      </button>
    </div>

    {#if tab === "files" && subsumedCount > 0}
      <label class="flex items-center gap-2 pb-2 text-xs text-muted-foreground">
        <Switch label="Hide sets inside duplicate folders" checked={hideSubsumed} onCheckedChange={(v) => (hideSubsumed = v)} />
        Hide {pluralize(subsumedCount, "set")} inside duplicate folders
      </label>
    {/if}
  </div>

  <!-- Lists -->
  {#if tab === "folders"}
    {#if report.folderGroups.length === 0}
      <p class="py-12 text-center text-sm text-muted-foreground">No duplicate folders found.</p>
    {:else}
      <div class="flex flex-col gap-2">
        {#each report.folderGroups as g (g.id)}
          <FolderGroup group={g} />
        {/each}
      </div>
    {/if}
  {:else if visibleFileGroups.length === 0}
    <p class="py-12 text-center text-sm text-muted-foreground">No duplicate files to show.</p>
  {:else}
    <div class="flex flex-col gap-2">
      {#each visibleFileGroups as g (g.id)}
        <FileGroup group={g} />
      {/each}
    </div>
  {/if}
</div>
