<script lang="ts">
  import { ChevronRight, FileIcon, Loader2, ShieldCheck, ShieldQuestion, FolderTree } from "lucide-svelte";
  import Badge from "$lib/components/ui/Badge.svelte";
  import { api } from "$lib/api";
  import { formatBytes, formatDate, dirOf, pluralize } from "$lib/format";
  import type { FileGroupDTO } from "$lib/types";

  let { group }: { group: FileGroupDTO } = $props();

  let open = $state(false);
  let verifying = $state(false);
  let verified = $state(false);
  // After verification a candidate may split into several confirmed subgroups.
  let confirmedGroups = $state<FileGroupDTO[]>([]);
  let verifyError = $state<string | null>(null);

  async function toggle() {
    open = !open;
    if (open && !verified && !verifying) {
      await verify();
    }
  }

  async function verify() {
    verifying = true;
    verifyError = null;
    try {
      const res = await api.verifyGroup({
        size: group.size,
        paths: group.files.map((f) => f.path),
      });
      confirmedGroups = res.groups ?? [];
      if (res.errors && Object.keys(res.errors).length > 0) {
        verifyError = Object.values(res.errors)[0];
      }
      verified = true;
    } catch (e) {
      verifyError = e instanceof Error ? e.message : String(e);
    } finally {
      verifying = false;
    }
  }

  // Subgroups to render: confirmed ones once verified, else the raw candidate.
  const subgroups = $derived(verified ? confirmedGroups : [group]);
</script>

<div class="rounded-lg border bg-card">
  <button
    type="button"
    onclick={toggle}
    class="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-accent/40"
  >
    <ChevronRight
      class="size-4 shrink-0 text-muted-foreground transition-transform {open ? 'rotate-90' : ''}"
    />
    <FileIcon class="size-4 shrink-0 text-muted-foreground" />
    <span class="min-w-0 flex-1 truncate font-medium">
      {group.files[0]?.name ?? "—"}
    </span>

    {#if group.withinDupFolder}
      <Badge variant="outline" class="gap-1">
        <FolderTree class="size-3" /> in dup folder
      </Badge>
    {/if}

    {#if verifying}
      <Badge variant="secondary" class="gap-1">
        <Loader2 class="size-3 animate-spin" /> verifying
      </Badge>
    {:else if verified}
      <Badge variant="success" class="gap-1">
        <ShieldCheck class="size-3" /> confirmed
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
      {formatBytes(group.reclaimable)} saved
    </span>
  </button>

  {#if open}
    <div class="border-t px-4 py-3">
      {#if verifyError}
        <p class="mb-2 text-sm text-destructive">Verification issue: {verifyError}</p>
      {/if}

      {#if verifying}
        <p class="text-sm text-muted-foreground">Comparing file contents…</p>
      {:else if verified && confirmedGroups.length === 0}
        <p class="text-sm text-muted-foreground">
          No confirmed duplicates — these files share a size and header but differ in content.
        </p>
      {:else}
        {#each subgroups as sg, i}
          {#if subgroups.length > 1}
            <p class="mb-1 mt-3 text-xs font-semibold uppercase text-muted-foreground">
              Confirmed set {i + 1} · {pluralize(sg.files.length, "copy", "copies")}
            </p>
          {/if}
          <ul class="divide-y rounded-md border">
            {#each sg.files as file (file.path)}
              <li class="flex items-center gap-3 px-3 py-2 text-sm">
                <span class="min-w-0 flex-1">
                  <span class="block truncate font-medium">{file.name}</span>
                  <span class="block truncate text-xs text-muted-foreground">{dirOf(file.path)}</span>
                </span>
                <span class="shrink-0 text-xs text-muted-foreground">{formatDate(file.modTime)}</span>
              </li>
            {/each}
          </ul>
        {/each}
      {/if}
    </div>
  {/if}
</div>
