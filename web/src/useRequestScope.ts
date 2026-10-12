import { useEffect, useMemo } from "react";

// One mounted query owns its reads. Writes to the same entity run in order;
// focus reads during writes are deferred until the whole batch settles.
export function useRequestScope(key: string) {
  const scope = useMemo(() => {
    let active = false;
    let generation = 0;
    let version = 0;
    let writes = 0;
    let waitingReads: (() => void)[] = [];
    let deferredRead: (() => void) | undefined;
    const queues = new Map<string, Promise<unknown>>();
    const current = (started: number) => active && generation === started;
    const read = async <T,>(request: () => Promise<T>, success: (value: T) => void, failure: (cause: Error) => void) => {
      if (!active) return;
      if (writes) return new Promise<void>((resolve) => {
        waitingReads.push(resolve);
        deferredRead = () => {
          const waiting = waitingReads;
          waitingReads = [];
          void read(request, success, failure).finally(() => waiting.forEach((done) => done()));
        };
      });
      const started = generation;
      const readVersion = ++version;
      try {
        const value = await request();
        if (current(started) && readVersion === version) success(value);
      } catch (cause) {
        if (current(started) && readVersion === version) failure(cause as Error);
      }
    };
    return {
      // key defines the lifetime; keeping it on the scope makes that ownership explicit.
      key,
      activate() { active = true; },
      dispose() {
        active = false; generation++; version++; deferredRead = undefined;
        waitingReads.forEach((done) => done()); waitingReads = [];
      },
      invalidate() { version++; },
      isActive() { return active; },
      read,
      async write<T>(entity: string | string[], request: () => Promise<T>, success: (value: T) => void, failure: (cause: Error) => void) {
        const started = generation;
        if (!current(started)) return false;
        version++;
        writes++;
        const entities = typeof entity === "string" ? [entity] : entity;
        const previous = Promise.all(entities.map((id) => queues.get(id)));
        const operation = previous.then(async () => {
          if (!current(started)) return false;
          try {
            const value = await request();
            if (current(started)) success(value);
            return current(started);
          } catch (cause) {
            if (current(started)) failure(cause as Error);
            return false;
          }
        });
        entities.forEach((id) => queues.set(id, operation));
        try { return await operation; }
        finally {
          entities.forEach((id) => { if (queues.get(id) === operation) queues.delete(id); });
          writes--;
          version++;
          if (!writes && current(started)) {
            const refresh = deferredRead;
            deferredRead = undefined;
            refresh?.();
          }
        }
      },
    };
  }, [key]);
  useEffect(() => { scope.activate(); return () => scope.dispose(); }, [scope]);
  return scope;
}
