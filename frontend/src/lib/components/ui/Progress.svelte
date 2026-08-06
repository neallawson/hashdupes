<script lang="ts">
  import { cn } from "$lib/utils";

  let {
    value = 0,
    max = 100,
    indeterminate = false,
    class: className = "",
  }: {
    value?: number;
    max?: number;
    indeterminate?: boolean;
    class?: string;
  } = $props();

  const pct = $derived(
    max > 0 ? Math.min(100, Math.max(0, (value / max) * 100)) : 0,
  );
</script>

<div
  class={cn(
    "relative h-2 w-full overflow-hidden rounded-full bg-secondary",
    className,
  )}
>
  {#if indeterminate}
    <div class="absolute inset-y-0 left-0 w-1/3 animate-pulse rounded-full bg-primary"></div>
  {:else}
    <div
      class="h-full rounded-full bg-primary transition-all"
      style="width: {pct}%"
    ></div>
  {/if}
</div>
