<script lang="ts">
  import { cn } from "$lib/utils";

  let {
    checked = false,
    disabled = false,
    id = undefined,
    label = undefined,
    onCheckedChange,
    class: className = "",
  }: {
    checked?: boolean;
    disabled?: boolean;
    id?: string;
    /** Accessible name. Omit when an external <label for> supplies one. */
    label?: string;
    onCheckedChange?: (checked: boolean) => void;
    class?: string;
  } = $props();
</script>

<!--
  A real checkbox sits transparent over the track so that keyboard focus,
  activation, and association with an external <label for> all come for free;
  the visible track is styled from its :checked / :focus-visible state.
-->
<span class={cn("relative inline-flex h-5 w-9 shrink-0", className)}>
  <input
    {id}
    {disabled}
    {checked}
    type="checkbox"
    role="switch"
    aria-label={label}
    onchange={(e) => onCheckedChange?.(e.currentTarget.checked)}
    class="peer absolute inset-0 z-10 m-0 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
  />
  <span
    class={cn(
      "pointer-events-none inline-flex h-5 w-9 items-center rounded-full border-2 border-transparent transition-colors peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-background peer-disabled:opacity-50",
      checked ? "bg-primary" : "bg-input",
    )}
  >
    <span
      class={cn(
        "block size-4 rounded-full bg-background shadow-lg transition-transform",
        checked ? "translate-x-4" : "translate-x-0",
      )}
    ></span>
  </span>
</span>
