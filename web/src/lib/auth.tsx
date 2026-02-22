import { useState, useEffect, useCallback, type ReactNode } from "react";
import { useNavigate } from "react-router";
import {
  authApi,
  setTokens,
  loadTokens,
  clearTokens,
  getAccessToken,
  type User,
} from "./api";
import { AuthContext, useAuth } from "./use-auth";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(() => {
    loadTokens();
    return !!getAccessToken();
  });
  const [sessionId, setSessionId] = useState<string | null>(
    () => localStorage.getItem("session_id"),
  );

  useEffect(() => {
    const token = getAccessToken();
    if (!token) return;

    authApi
      .getMe()
      .then(setUser)
      .catch(() => {
        clearTokens();
        setUser(null);
      })
      .finally(() => setIsLoading(false));
  }, []);

  const login = useCallback(async (email: string) => {
    await authApi.webLogin(email);
  }, []);

  const verify = useCallback(async (token: string) => {
    const result = await authApi.webVerify(token);
    setTokens(result.access_token, result.refresh_token);
    localStorage.setItem("session_id", result.session_id);
    setSessionId(result.session_id);
    const me = await authApi.getMe();
    setUser(me);
  }, []);

  const logout = useCallback(() => {
    clearTokens();
    setUser(null);
    setSessionId(null);
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isAuthenticated: !!user,
        isLoading,
        sessionId,
        login,
        verify,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function AuthGuard({ children }: { children: ReactNode }) {
  const { user, isAuthenticated, isLoading } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      navigate("/login", { replace: true });
    }
    if (!isLoading && isAuthenticated && user?.needs_username) {
      navigate("/setup-username", { replace: true });
    }
  }, [isLoading, isAuthenticated, user, navigate]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <div className="text-zinc-500">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated || user?.needs_username) return null;
  return <>{children}</>;
}
