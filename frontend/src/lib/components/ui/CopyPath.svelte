<script lang="ts">
  import { Check, Clipboard } from "lucide-svelte";
  import { cn } from "$lib/utils";
  import { copyText } from "$lib/clipboard";

  let {
    path,
    class: className = "",
  }: { path: string; class?: string } = $props();

  let copied = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;

  async function copy() {
    if (await copyText(path)) {
      copied = true;
      clearTimeout(timer);
      timer = setTimeout(() => (copied = false), 1200);
    }
  }
</script>

<button
  type="button"
  onclick={copy}
  title={copied ? "Copied" : `Copy ${path}`}
  aria-label={copied ? "Path copied" : "Copy path"}
  class={cn(
    "inline-flex size-6 shrink-0 items-center justify-center rounded text-muted-foreground transition-colors hover:bg-accent hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring",
    className,
  )}
>
  {#if copied}
    <Check class="size-3.5 text-emerald-500" />
  {:else}
    <Clipboard class="size-3.5" />
  {/if}
</button>
