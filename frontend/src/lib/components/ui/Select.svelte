<script lang="ts" generics="T extends string">
  import { ChevronDown } from "lucide-svelte";
  import { cn } from "$lib/utils";

  let {
    value = $bindable(),
    options,
    id = undefined,
    ariaLabel = undefined,
    class: className = "",
  }: {
    value: T;
    options: ReadonlyArray<{ value: T; label: string }>;
    id?: string;
    ariaLabel?: string;
    class?: string;
  } = $props();
</script>

<!--
  A native select wrapped to centralize one platform quirk: WebKit ignores an
  inherited text color on selects and their options, so both must set it
  explicitly or the text is invisible on the dark theme.
-->
<div class={cn("relative", className)}>
  <select
    {id}
    aria-label={ariaLabel}
    bind:value
    class="h-9 w-full appearance-none rounded-md border border-input bg-background px-3 pr-8 text-sm text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
  >
    {#each options as opt (opt.value)}
      <option class="bg-background text-foreground" value={opt.value}>
        {opt.label}
      </option>
    {/each}
  </select>
  <ChevronDown
    class="pointer-events-none absolute right-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
  />
</div>
