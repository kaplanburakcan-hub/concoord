import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { Can, RequireAuth, RequirePerm } from "./guards";

type FakeAuth = { user: { id: string } | null; loading: boolean; perms: string[] };
let auth: FakeAuth;

vi.mock("./AuthContext", () => ({
  useAuth: () => ({
    user: auth.user,
    loading: auth.loading,
    can: (p: string) => auth.perms.includes(p),
  }),
}));

beforeEach(() => {
  auth = { user: { id: "u1" }, loading: false, perms: [] };
});

describe("Can", () => {
  it("izin varsa çocukları gösterir", () => {
    auth.perms = ["admin.manage_users"];
    render(<Can perm="admin.manage_users">Kullanıcılar</Can>);
    expect(screen.getByText("Kullanıcılar")).toBeInTheDocument();
  });

  it("izin yoksa fallback gösterir", () => {
    render(
      <Can perm="admin.manage_users" fallback={<span>Yetkisiz</span>}>
        Kullanıcılar
      </Can>
    );
    expect(screen.queryByText("Kullanıcılar")).not.toBeInTheDocument();
    expect(screen.getByText("Yetkisiz")).toBeInTheDocument();
  });
});

describe("RequirePerm", () => {
  it("izin yoksa 403 ekranı ve gerekli izni gösterir", () => {
    render(<RequirePerm perm="progress_payments.approve">Hakediş</RequirePerm>);
    expect(screen.getByText("Bu sayfa için yetkiniz yok.")).toBeInTheDocument();
    expect(screen.getByText("Gerekli izin: progress_payments.approve")).toBeInTheDocument();
    expect(screen.queryByText("Hakediş")).not.toBeInTheDocument();
  });

  it("yüklenirken içeriği göstermez", () => {
    auth.loading = true;
    auth.perms = ["progress_payments.approve"];
    render(<RequirePerm perm="progress_payments.approve">Hakediş</RequirePerm>);
    expect(screen.getByText("Yükleniyor…")).toBeInTheDocument();
    expect(screen.queryByText("Hakediş")).not.toBeInTheDocument();
  });

  it("izin varsa içeriği gösterir", () => {
    auth.perms = ["progress_payments.approve"];
    render(<RequirePerm perm="progress_payments.approve">Hakediş</RequirePerm>);
    expect(screen.getByText("Hakediş")).toBeInTheDocument();
  });
});

describe("RequireAuth", () => {
  function renderAt(path: string) {
    render(
      <MemoryRouter
        initialEntries={[path]}
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
      >
        <Routes>
          <Route path="/login" element={<p>Giriş sayfası</p>} />
          <Route
            path="/projects"
            element={
              <RequireAuth>
                <p>Projeler</p>
              </RequireAuth>
            }
          />
        </Routes>
      </MemoryRouter>
    );
  }

  it("oturum yoksa /login'e yönlendirir", () => {
    auth.user = null;
    renderAt("/projects");
    expect(screen.getByText("Giriş sayfası")).toBeInTheDocument();
    expect(screen.queryByText("Projeler")).not.toBeInTheDocument();
  });

  it("oturum varsa sayfayı gösterir", () => {
    renderAt("/projects");
    expect(screen.getByText("Projeler")).toBeInTheDocument();
  });
});
