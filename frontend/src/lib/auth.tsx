import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { api, getToken, setToken } from "./api";

export type User = {
  username: string;
  email: string;
  displayName: string;
  bio: string;
  avatarUrl: string;
  createdAt: string;
};

type AuthCtx = {
  user: User | null;
  loading: boolean;
  login: (identifier: string, password: string) => Promise<void>;
  register: (
    email: string,
    username: string,
    password: string,
  ) => Promise<void>;
  logout: () => void;
  refresh: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState<boolean>(!!getToken());

  const refresh = useCallback(async () => {
    if (!getToken()) {
      setUser(null);
      setLoading(false);
      return;
    }
    try {
      const me = await api<User>("GET", "/users/me");
      setUser(me);
    } catch {
      setUser(null);
      setToken(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  useEffect(() => {
    function onUnauthenticated() {
      setUser(null);
      setLoading(false);
    }
    window.addEventListener("trippy:unauthenticated", onUnauthenticated);
    return () =>
      window.removeEventListener("trippy:unauthenticated", onUnauthenticated);
  }, []);

  const login: AuthCtx["login"] = useCallback(async (identifier, password) => {
    const res = await api<{ token: string; user: User }>(
      "POST",
      "/auth/login",
      { identifier, password },
    );
    setToken(res.token);
    setUser(res.user);
  }, []);

  const register: AuthCtx["register"] = useCallback(
    async (email, username, password) => {
      const res = await api<{ token: string; user: User }>(
        "POST",
        "/auth/register",
        { email, username, password },
      );
      setToken(res.token);
      setUser(res.user);
    },
    [],
  );

  const logout = useCallback(() => {
    setToken(null);
    setUser(null);
  }, []);

  const value = useMemo(
    () => ({ user, loading, login, register, logout, refresh }),
    [user, loading, login, register, logout, refresh],
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth() {
  const v = useContext(Ctx);
  if (!v) throw new Error("useAuth must be used within AuthProvider");
  return v;
}
