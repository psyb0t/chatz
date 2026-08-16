import {
  getAuthStatus,
  login as apiLogin,
  logout as apiLogout,
  setup as apiSetup,
  type Credentials,
  type User,
} from "$lib/api/client";
import {
  PHASE_ANON,
  PHASE_AUTHED,
  PHASE_LOADING,
  PHASE_SETUP,
  type AuthPhase,
} from "$lib/common/auth";
import { log } from "$lib/log";
import {
  EVENT_AUTH_LOGIN,
  EVENT_AUTH_LOGOUT,
  EVENT_AUTH_PHASE,
  EVENT_AUTH_SETUP,
} from "$lib/common/log-events";

// AuthStore holds the current auth phase + user. refresh() re-derives the phase
// from GET /auth/status; setup/login/logout mutate server-side then refresh so
// the phase always reflects the server's truth (the cookie).
class AuthStore {
  phase = $state<AuthPhase>(PHASE_LOADING);
  user = $state<User | null>(null);

  // setPhase is the single write-path for phase so every transition emits an
  // auth.phase log line (from -> to). No credentials ever pass through here.
  private setPhase(next: AuthPhase): void {
    if (next === this.phase) {
      return;
    }

    log.info(EVENT_AUTH_PHASE, { from: this.phase, to: next });
    this.phase = next;
  }

  async refresh(): Promise<void> {
    const status = await getAuthStatus();

    if (status.needsSetup) {
      this.setPhase(PHASE_SETUP);
      this.user = null;

      return;
    }

    if (status.authenticated && status.user) {
      this.setPhase(PHASE_AUTHED);
      this.user = status.user;

      return;
    }

    this.setPhase(PHASE_ANON);
    this.user = null;
  }

  async setup(credentials: Credentials): Promise<void> {
    log.info(EVENT_AUTH_SETUP);
    await apiSetup(credentials);
    await this.refresh();
  }

  async login(credentials: Credentials): Promise<void> {
    log.info(EVENT_AUTH_LOGIN);
    await apiLogin(credentials);
    await this.refresh();
  }

  async logout(): Promise<void> {
    log.info(EVENT_AUTH_LOGOUT);
    await apiLogout();
    await this.refresh();
  }
}

export const auth = new AuthStore();
