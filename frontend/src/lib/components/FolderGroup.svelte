<script lang="ts">
  import { ChevronRight, Folder, FolderOpen, Files } from "lucide-svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import CopyPath from "$lib/components/ui/CopyPath.svelte";
  import { formatBytes, dirOf, pluralize } from "$lib/format";
  import type { FolderGroupDTO } from "$lib/types";

  let { group }: { group: FolderGroupDTO } = $props();

  let open = $state(false);
  const first = $derived(group.folders[0]);
  // Members are content-identical by definition, so the first member's counts
  // describe them all.
  const isEmpty = $derived(first?.isEmpty ?? false);
</script>

<div class="rounded-lg border bg-card">
  <button
    type="button"
    onclick={() => (open = !open)}
    aria-expanded={open}
    class="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-accent/40"
  >
    <ChevronRight
      class="size-4 shrink-0 text-muted-foreground transition-transform {open
        ? 'rotate-90'
        : ''}"
    />
    <Folder class="size-4 shrink-0 text-muted-foreground" />
    <span class="min-w-0 flex-1 truncate font-medium">{first?.name ?? "—"}</span>

    <Badge variant="outline" class="gap-1">
      {pluralize(group.folders.length, "location")}
    </Badge>

    {#if isEmpty}
      <Badge variant="outline" class="gap-1 border-amber-500/40 text-amber-500">
        <FolderOpen class="size-3" /> empty
      </Badge>
    {:else}
      <Badge variant="secondary" class="gap-1">
        <Files class="size-3" /> {pluralize(first?.fileCount ?? 0, "file")}
      </Badge>
    {/if}

    <span class="w-20 text-right text-sm tabular-nums text-muted-foreground">
      {formatBytes(first?.totalSize ?? 0)}
    </span>
    <span class="w-28 text-right text-sm tabular-nums">
      {formatBytes(group.reclaimable)} saved
    </span>
  </button>

  {#if open}
    <div class="border-t px-4 py-3">
      <ul class="divide-y rounded-md border">
        {#each group.folders as folder (folder.path)}
          <li class="flex items-center gap-2 px-3 py-2 text-sm">
            <Folder class="size-4 shrink-0 text-muted-foreground" />
            <span class="min-w-0 flex-1">
              <span class="block truncate font-medium">{folder.name}</span>
              <span class="block truncate text-xs text-muted-foreground">
                {dirOf(folder.path)}
              </span>
            </span>
            <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
              {folder.isEmpty
                ? "empty"
                : `${pluralize(folder.fileCount, "file")} · ${formatBytes(folder.totalSize)}`}
            </span>
            <CopyPath path={folder.path} />
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</div>
