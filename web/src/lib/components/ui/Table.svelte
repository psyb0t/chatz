<script lang="ts">
  interface Props {
    columns: string[];
    rows: string[][];
    id?: string;
    testid?: string;
    // Extra attributes for the scrolling root wrap (e.g. the json-render
    // data-jr-type stamp). Forwarded verbatim onto the outermost element.
    rootAttrs?: Record<string, string>;
  }

  const { columns, rows, id, testid, rootAttrs = {} }: Props = $props();
</script>

<div class="table__wrap" {id} data-testid={testid} {...rootAttrs}>
  <table class="table">
    <thead>
      <tr>
        {#each columns as col, i (i)}
          <th>{col}</th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each rows as row, r (r)}
        <tr>
          {#each row as cell, c (c)}
            <td>{cell}</td>
          {/each}
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .table__wrap {
    min-width: 0;
    max-width: 100%;
    overflow-x: auto;
    overscroll-behavior-inline: contain;
    border: var(--border-width) solid var(--border);
    border-radius: var(--radius);
    background: var(--panel);
  }

  .table {
    border-collapse: collapse;
    width: 100%;
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }

  .table th,
  .table td {
    border-bottom: var(--border-width) solid var(--border);
    padding: var(--space-2) var(--space-3);
    text-align: left;
    white-space: nowrap;
  }

  .table th {
    background: var(--panel-2);
    color: var(--muted);
    font-family: var(--font-display);
    font-weight: 600;
    font-size: var(--text-xs);
  }

  .table tbody tr:last-child td {
    border-bottom: none;
  }
</style>
