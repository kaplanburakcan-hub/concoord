import { beforeEach, describe, expect, it, vi } from "vitest";
import { RequestError } from "../api/client";

const apiMock = vi.fn();
vi.mock("../api/client", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../api/client")>()),
  api: (...args: unknown[]) => apiMock(...args),
}));

const { apiWithOfflineFallback, enqueue, queued, remove, sync } = await import("./queue");

function setOnline(v: boolean) {
  Object.defineProperty(navigator, "onLine", { configurable: true, get: () => v });
}

const base = { projectId: "p1", body: { a: 1 }, label: "Günlük rapor" } as const;

beforeEach(() => {
  apiMock.mockReset();
  setOnline(true);
});

describe("offline kuyruk", () => {
  it("enqueue/remove localStorage'da kalıcıdır", () => {
    const item = enqueue({ ...base, method: "POST", path: "/projects/p1/daily-reports" });
    expect(item.status).toBe("pending");
    expect(queued()).toHaveLength(1);
    remove(item.id);
    expect(queued()).toHaveLength(0);
  });

  it("çevrimdışıyken isteği kuyruğa alır", async () => {
    setOnline(false);
    const res = await apiWithOfflineFallback({ ...base, method: "POST", path: "/x" });
    expect(res).toEqual({ queued: true });
    expect(apiMock).not.toHaveBeenCalled();
    expect(queued()).toHaveLength(1);
  });

  it("ağ hatasında kuyruğa alır, sunucu hatasını yukarı fırlatır", async () => {
    apiMock.mockRejectedValueOnce(new TypeError("fetch failed"));
    await expect(apiWithOfflineFallback({ ...base, method: "POST", path: "/x" })).resolves.toEqual({
      queued: true,
    });

    apiMock.mockRejectedValueOnce(new RequestError(422));
    await expect(
      apiWithOfflineFallback({ ...base, method: "POST", path: "/x" })
    ).rejects.toBeInstanceOf(RequestError);
    expect(queued()).toHaveLength(1);
  });

  it("sync sırayı korur ve {local:id} yer tutucusunu sunucu id'siyle değiştirir", async () => {
    const created = enqueue({ ...base, method: "POST", path: "/projects/p1/daily-reports" });
    enqueue({ ...base, method: "PUT", path: `/daily-reports/{local:${created.id}}` });
    enqueue({ ...base, method: "POST", path: `/daily-reports/{local:${created.id}}/submit` });

    apiMock.mockResolvedValueOnce({ id: "srv-42" }).mockResolvedValue({});
    await expect(sync()).resolves.toEqual({ done: 3, remaining: 0 });

    expect(apiMock.mock.calls.map((c) => c[0])).toEqual([
      "/projects/p1/daily-reports",
      "/daily-reports/srv-42",
      "/daily-reports/srv-42/submit",
    ]);
  });

  it("409'da öğeyi conflict işaretler ve arkasındakileri bekletir", async () => {
    enqueue({ ...base, method: "POST", path: "/a" });
    enqueue({ ...base, method: "POST", path: "/b" });
    apiMock.mockRejectedValueOnce(new RequestError(409, { code: "conflict", message: "Çakışma" }));

    await expect(sync()).resolves.toEqual({ done: 0, remaining: 2 });
    const [first, second] = queued();
    expect(first.status).toBe("conflict");
    expect(first.error).toBe("Çakışma");
    expect(second.status).toBe("pending");

    // Conflict öğesi önde durdukça kuyruk ilerlemez.
    await sync();
    expect(apiMock).toHaveBeenCalledOnce();
  });

  it("5xx'te öğeyi değiştirmeden bekletir (sonra yeniden denenir)", async () => {
    enqueue({ ...base, method: "POST", path: "/a" });
    apiMock.mockRejectedValueOnce(new RequestError(503));
    await expect(sync()).resolves.toEqual({ done: 0, remaining: 1 });
    expect(queued()[0].status).toBe("pending");

    apiMock.mockResolvedValueOnce({});
    await expect(sync()).resolves.toEqual({ done: 1, remaining: 0 });
  });
});
