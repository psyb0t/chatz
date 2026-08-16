import {
  listUsers,
  createUser as apiCreateUser,
  deleteUser as apiDeleteUser,
  type User,
  type CreateUserRequest,
} from "$lib/api/client";
import { log } from "$lib/log";
import {
  EVENT_ADMIN_USER_CREATE,
  EVENT_ADMIN_USER_DELETE,
  EVENT_USERS_ERROR,
  EVENT_USERS_LOADED,
} from "$lib/common/log-events";

// UsersStore holds the admin user list. load() is called by the admin users page
// on mount (admin-only). create/remove mutate server-side then re-load so the
// list always reflects the server's truth. Passwords never touch the store's
// state or its log lines — only the request body openapi-fetch sends.
class UsersStore {
  list = $state<User[]>([]);
  loaded = $state(false);
  loading = $state(false);
  // error holds the last load failure's message for inline display; cleared on
  // the next load attempt. Mutation errors surface on the page's own form state.
  error = $state<string | null>(null);

  async load(): Promise<void> {
    this.loading = true;
    this.error = null;

    try {
      this.list = await listUsers();
      this.loaded = true;
      log.info(EVENT_USERS_LOADED, { count: this.list.length });
    } catch (err) {
      this.error = err instanceof Error ? err.message : String(err);
      log.error(EVENT_USERS_ERROR, { message: this.error });
    } finally {
      this.loading = false;
    }
  }

  // create provisions a user then re-loads. It logs only the username + admin
  // flag — NEVER the password (which lives only in the request body).
  async create(body: CreateUserRequest): Promise<void> {
    const created = await apiCreateUser(body);
    log.info(EVENT_ADMIN_USER_CREATE, {
      id: created.id,
      username: created.username,
      isAdmin: created.isAdmin,
    });
    await this.load();
  }

  async remove(userId: string): Promise<void> {
    await apiDeleteUser(userId);
    log.info(EVENT_ADMIN_USER_DELETE, { id: userId });
    await this.load();
  }
}

export const users = new UsersStore();
