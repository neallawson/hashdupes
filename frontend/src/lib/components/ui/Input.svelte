<script lang="ts">
  import { cn } from "$lib/utils";

  let {
    value = $bindable(""),
    id = undefined,
    placeholder = undefined,
    type = "text",
    disabled = false,
    ariaLabel = undefined,
    class: className = "",
  }: {
    value?: string;
    id?: string;
    placeholder?: string;
    type?: "text" | "search";
    disabled?: boolean;
    ariaLabel?: string;
    class?: string;
  } = $props();
</script>

<input
  {id}
  {type}
  {placeholder}
  {disabled}
  aria-label={ariaLabel}
  bind:value
  spellcheck="false"
  class={cn(
    "h-9 min-w-0 rounded-md border border-input bg-background px-3 text-sm text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50",
    className,
  )}
/>
