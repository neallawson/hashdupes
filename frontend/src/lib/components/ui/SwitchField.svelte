<script lang="ts">
  import { cn } from "$lib/utils";
  import Switch from "$lib/components/ui/Switch.svelte";

  let {
    label,
    checked = false,
    disabled = false,
    onCheckedChange,
    class: className = "",
    labelClass = "text-sm",
  }: {
    label: string;
    checked?: boolean;
    disabled?: boolean;
    onCheckedChange?: (checked: boolean) => void;
    class?: string;
    labelClass?: string;
  } = $props();

  const id = `switch-${nextId()}`;
</script>

<script lang="ts" module>
  let counter = 0;
  function nextId(): number {
    return ++counter;
  }
</script>

<div class={cn("flex items-center gap-2", className)}>
  <Switch {id} {checked} {disabled} {onCheckedChange} />
  <label
    for={id}
    class={cn(
      "cursor-pointer select-none",
      disabled && "cursor-not-allowed opacity-50",
      labelClass,
    )}
  >
    {label}
  </label>
</div>
