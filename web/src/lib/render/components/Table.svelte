<script lang="ts">
  import type { BaseComponentProps } from "@json-render/svelte";
  import { COMP_TABLE, DATA_JR_TYPE } from "$lib/common/render";
  import Table from "$lib/components/ui/Table.svelte";

  type TableProps = {
    id?: string | null;
    columns?: string[] | null;
    rows?: string[][] | null;
  };

  const { props }: BaseComponentProps<TableProps> = $props();

  const columns = $derived(Array.isArray(props.columns) ? props.columns : []);
  const rows = $derived(Array.isArray(props.rows) ? props.rows : []);
</script>

<Table
  {columns}
  {rows}
  id={props.id ?? undefined}
  rootAttrs={{ [DATA_JR_TYPE]: COMP_TABLE }}
/>
