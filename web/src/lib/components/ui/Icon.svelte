<script lang="ts" module>
  // Stroke icons on a 24px grid, drawn with currentColor so they follow the
  // text color of whatever holds them. Each entry is one or more SVG path
  // strings; the set is closed so markup can't inject arbitrary SVG.
  const PATHS = {
    "arrow-up": ["M12 19V5", "M5 12l7-7 7 7"],
    stop: ["M7 7h10v10H7z"],
    sliders: [
      "M4 7h10",
      "M18 7h2",
      "M4 17h4",
      "M12 17h8",
      "M16 5v4",
      "M10 15v4",
    ],
    brain: [
      "M9.5 4a2.5 2.5 0 0 0-2.5 2.5v.2A3 3 0 0 0 5 9.5a3 3 0 0 0 .8 2A3 3 0 0 0 5 14a3 3 0 0 0 2.5 3A2.5 2.5 0 0 0 10 20h2V4z",
      "M14.5 4A2.5 2.5 0 0 1 17 6.5v.2a3 3 0 0 1 2 2.8 3 3 0 0 1-.8 2A3 3 0 0 1 19 14a3 3 0 0 1-2.5 3A2.5 2.5 0 0 1 14 20h-2",
    ],
    "chevron-down": ["M6 9l6 6 6-6"],
    "chevron-right": ["M9 6l6 6-6 6"],
    check: ["M5 12.5l4.5 4.5L19 7.5"],
    x: ["M6 6l12 12", "M18 6L6 18"],
    wrench: [
      "M14.7 6.3a4 4 0 0 0-5.4 5.2L4 16.8V20h3.2l5.3-5.3a4 4 0 0 0 5.2-5.4l-2.6 2.6-2.4-.5-.5-2.4z",
    ],
    copy: ["M9 9h10v10H9z", "M5 15V5h10"],
    plug: ["M9 3v5", "M15 3v5", "M6 8h12v3a6 6 0 0 1-12 0z", "M12 17v4"],
    plus: ["M12 5v14", "M5 12h14"],
    search: ["M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14z", "M20 20l-4-4"],
    sparkle: [
      "M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z",
      "M19 16l.7 1.8 1.8.7-1.8.7L19 21l-.7-1.8-1.8-.7 1.8-.7z",
    ],
    reset: ["M4 12a8 8 0 1 0 2.4-5.7", "M4 4v4h4"],
  } as const;

  export type IconName = keyof typeof PATHS;
</script>

<script lang="ts">
  interface Props {
    name: IconName;
    size?: number;
    strokeWidth?: number;
    spin?: boolean;
  }

  const { name, size = 16, strokeWidth = 1.8, spin = false }: Props = $props();
</script>

<svg
  class="icon"
  class:icon--spin={spin}
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill="none"
  stroke="currentColor"
  stroke-width={strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  {#each PATHS[name] as d (d)}
    <path {d} />
  {/each}
</svg>

<style>
  .icon {
    flex-shrink: 0;
  }

  .icon--spin {
    animation: spin 0.9s linear infinite;
  }
</style>
