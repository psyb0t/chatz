<script lang="ts">
  import { goto } from "$app/navigation";
  import { ROUTE_ADMIN_USERS } from "$lib/common/routes";

  // /admin has no content of its own; the sidebar gear points here and it lands
  // on the first tab (Users). replaceState keeps it out of the back stack.
  $effect(() => {
    void goto(ROUTE_ADMIN_USERS, { replaceState: true });
  });
</script>
