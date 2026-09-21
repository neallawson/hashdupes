<script lang="ts">
  import {
    ChevronRight,
    FileIcon,
    Loader2,
    ShieldCheck,
    ShieldQuestion,
    ShieldAlert,
    ShieldX,
    Split,
    FolderTree,
    CircleSlash,
  } from "lucide-svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import CopyPath from "$lib/components/ui/CopyPath.svelte";
  import { formatBytes, formatDate, dirOf, pluralize } from "$lib/format";
  import type { FileGroupDTO } from "$lib/types";
  import type { Verification } from "$lib/verification.svelte";

  let {
    group,
    verification,
  }: { group: FileGroupDTO; verification: Verification } = $props();

  let open = $state(false);

  const result = $derived(verification.result(group.id));
  const isZeroByte = $derived(group.size === 0);
  // Confirmed subgroups once verified, otherwise the raw candidate.
  const subgroups = $derived(
    result.outcome === "confirmed" || result.outcome === "split"
      ? result.groups
      : [group],
  );
  const reclaimable = $derived(verification.reclaimableFor(group));

  async function toggle() {
    open = !open;
    if (open) await verification.verify(group);
  }
</script>

<div class="rounded-lg border bg-card">
  <button
    type="button"
    onclick={toggle}
    aria-expanded={open}
    class="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-accent/40"
  >
    <ChevronRight
      class="size-4 shrink-0 text-muted-foreground transition-transform {open
        ? 'rotate-90'
        : ''}"
    />
    <FileIcon class="size-4 shrink-0 text-muted-foreground" />
    <span class="min-w-0 flex-1 truncate font-medium">
      {group.files[0]?.name ?? "—"}
    </span>

    <span class="shrink-0 text-xs text-muted-foreground">
      {pluralize(group.files.length, "copy", "copies")}
    </span>

    {#if isZeroByte}
      <Badge variant="outline" class="gap-1 border-amber-500/40 text-amber-500">
        <CircleSlash class="size-3" /> zero-byte
      </Badge>
    {/if}

    {#if group.withinDupFolder}
      <Badge variant="outline" class="gap-1">
        <FolderTree class="size-3" /> in dup folder
      </Badge>
    {/if}

    {#if result.outcome === "verifying"}
      <Badge variant="secondary" class="gap-1">
        <Loader2 class="size-3 animate-spin" /> verifying
      </Badge>
    {:else if result.outcome === "confirmed"}
      <Badge variant="success" class="gap-1">
        <ShieldCheck class="size-3" /> confirmed
      </Badge>
    {:else if result.outcome === "split"}
      <Badge variant="outline" class="gap-1 border-amber-500/40 text-amber-500">
        <Split class="size-3" /> split
      </Badge>
    {:else if result.outcome === "rejected"}
      <Badge variant="destructive" class="gap-1">
        <ShieldX class="size-3" /> not duplicates
      </Badge>
    {:else if result.outcome === "failed"}
      <Badge variant="destructive" class="gap-1">
        <ShieldAlert class="size-3" /> verify failed
      </Badge>
    {:else}
      <Badge variant="secondary" class="gap-1">
        <ShieldQuestion class="size-3" /> candidate
      </Badge>
    {/if}

    <span class="w-20 text-right text-sm tabular-nums text-muted-foreground">
      {formatBytes(group.size)}
    </span>
    <span class="w-28 text-right text-sm tabular-nums">
      {formatBytes(reclaimable)} saved
    </span>
  </button>

  {#if open}
    <div class="border-t px-4 py-3">
      {#if result.error}
        <p class="mb-2 text-sm text-destructive">
          Verification issue: {result.error}
        </p>
      {/if}

      {#if result.outcome === "verifying"}
        <p class="text-sm text-muted-foreground">Comparing file contents…</p>
      {:else if result.outcome === "rejected"}
        <p class="mb-3 text-sm text-muted-foreground">
          Not duplicates — these files share a size and content header but differ
          later in the file. This is the head-hash collision the byte-compare
          exists to catch.
        </p>
      {:else if result.outcome === "split"}
        <p class="mb-3 text-sm text-amber-500">
          The candidate split into {pluralize(result.groups.length, "confirmed set")}
          after comparing contents.
        </p>
      {/if}

      {#if result.outcome !== "verifying"}
        {#each subgroups as sg, i (sg.id + i)}
          {#if subgroups.length > 1}
            <p
              class="mb-1 mt-3 text-xs font-semibold uppercase text-muted-foreground"
            >
              Confirmed set {i + 1} · {pluralize(
                sg.files.length,
                "copy",
                "copies",
              )}
            </p>
          {/if}
          <ul class="divide-y rounded-md border">
            {#each sg.files as file (file.path)}
              <li class="flex items-center gap-2 px-3 py-2 text-sm">
                <span class="min-w-0 flex-1">
                  <span class="flex items-center gap-1.5">
                    <span class="truncate font-medium">{file.name}</span>
                    {#if file.isEmpty}
                      <CircleSlash
                        class="size-3 shrink-0 text-amber-500"
                        aria-label="zero-byte file"
                      />
                    {/if}
                  </span>
                  <span class="block truncate text-xs text-muted-foreground">
                    {dirOf(file.path)}
                  </span>
                </span>
                <span class="shrink-0 text-xs tabular-nums text-muted-foreground">
                  {formatDate(file.modTime)}
                </span>
                <CopyPath path={file.path} />
              </li>
            {/each}
          </ul>
        {/each}
      {/if}
    </div>
  {/if}
</div>
