import { listModels, type Model } from "$lib/api/client";
import { log } from "$lib/log";
import {
  EVENT_MODELS_ERROR,
  EVENT_MODELS_LOADED,
} from "$lib/common/log-events";

// ModelsStore holds the merged upstream model list for the composer picker.
// Empty when no upstream key is configured.
class ModelsStore {
  list = $state<Model[]>([]);
  loaded = $state(false);

  async load(): Promise<void> {
    try {
      this.list = await listModels();
      this.loaded = true;
      log.info(EVENT_MODELS_LOADED, { count: this.list.length });
    } catch (err) {
      log.error(EVENT_MODELS_ERROR, {
        message: err instanceof Error ? err.message : String(err),
      });
    }
  }
}

export const models = new ModelsStore();
