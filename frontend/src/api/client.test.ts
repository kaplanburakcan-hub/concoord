import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  api,
  describeLoadError,
  getRefreshToken,
  hasSession,
  login,
  RequestError,
  setAccessToken,
  setOnSessionLost,
  setRefreshToken,
} from "./client";

function jsonResponse(status: number, body?: unknown): Response {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  setAccessToken(null);
  setRefreshToken(null);
  setOnSessionLost(() => {});
});

describe("describeLoadError", () => {
  it("403'ü yetki hatası olarak gösterir", () => {
    expect(describeLoadError(new RequestError(403), "Projeler")).toBe(
      "Projeler için erişim yetkiniz yok."
    );
  });

  it("diğer HTTP hatalarında gerçek nedeni gösterir", () => {
    const err = new RequestError(500, { code: "internal", message: "Beklenmeyen bir hata oluştu." });
    expect(describeLoadError(err, "Projeler")).toBe(
      "Projeler yüklenemedi (Beklenmeyen bir hata oluştu.)."
    );
  });

  it("ağ hatasında bağlantı mesajı gösterir", () => {
    expect(describeLoadError(new TypeError("fetch failed"), "Projeler")).toBe(
      "Projeler yüklenemedi. Bağlantınızı kontrol edip tekrar deneyin."
    );
  });
});

describe("login", () => {
  it("token çiftini saklar ve kullanıcıyı döner", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(200, {
        user: { id: "u1", email: "a@b.c" },
        tokens: { access_token: "AT", refresh_token: "RT" },
      })
    );
    const user = await login("a@b.c", "pw");
    expect(user.id).toBe("u1");
    expect(getRefreshToken()).toBe("RT");
    expect(hasSession()).toBe(true);

    fetchMock.mockResolvedValueOnce(jsonResponse(200, { ok: true }));
    await api("/projects");
    const headers = fetchMock.mock.calls[1][1]!.headers as Record<string, string>;
    expect(headers.Authorization).toBe("Bearer AT");
  });

  it("hata zarfındaki mesajı RequestError'a taşır", async () => {
    fetchMock.mockResolvedValueOnce(
      jsonResponse(401, { error: { code: "unauthorized", message: "Parola hatalı." } })
    );
    await expect(login("a@b.c", "x")).rejects.toMatchObject({
      status: 401,
      message: "Parola hatalı.",
      api: { code: "unauthorized" },
    });
    expect(hasSession()).toBe(false);
  });
});

describe("api", () => {
  it("proje başlığını ve JSON gövdesini gönderir", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse(201, { id: "p1" }));
    const res = await api<{ id: string }>("/projects", {
      method: "POST",
      body: { name: "X" },
      projectId: "pid-1",
    });
    expect(res.id).toBe("p1");
    const [url, init] = fetchMock.mock.calls[0];
    expect(url).toBe("/api/v1/projects");
    expect(init!.method).toBe("POST");
    expect(init!.body).toBe(JSON.stringify({ name: "X" }));
    expect((init!.headers as Record<string, string>)["X-Project-Id"]).toBe("pid-1");
  });

  it("204'te undefined döner", async () => {
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await expect(api("/x", { method: "DELETE" })).resolves.toBeUndefined();
  });

  it("401'de bir kez refresh dener ve isteği yeniler", async () => {
    setRefreshToken("RT-old");
    fetchMock
      .mockResolvedValueOnce(jsonResponse(401))
      .mockResolvedValueOnce(
        jsonResponse(200, { tokens: { access_token: "AT-new", refresh_token: "RT-new" } })
      )
      .mockResolvedValueOnce(jsonResponse(200, { ok: true }));

    await expect(api("/projects")).resolves.toEqual({ ok: true });
    expect(fetchMock.mock.calls[1][0]).toBe("/api/v1/auth/refresh");
    expect(getRefreshToken()).toBe("RT-new");
    const retryHeaders = fetchMock.mock.calls[2][1]!.headers as Record<string, string>;
    expect(retryHeaders.Authorization).toBe("Bearer AT-new");
  });

  it("refresh başarısızsa oturumu düşürür", async () => {
    setRefreshToken("RT-old");
    const lost = vi.fn();
    setOnSessionLost(lost);
    fetchMock.mockResolvedValueOnce(jsonResponse(401)).mockResolvedValueOnce(jsonResponse(401));

    await expect(api("/projects")).rejects.toMatchObject({ status: 401 });
    expect(lost).toHaveBeenCalledOnce();
    expect(hasSession()).toBe(false);
  });

  it("hata gövdesi olmayan yanıtta HTTP durumunu mesaj yapar", async () => {
    fetchMock.mockResolvedValueOnce(new Response("not json", { status: 502 }));
    await expect(api("/projects")).rejects.toMatchObject({ status: 502, message: "HTTP 502" });
  });
});
