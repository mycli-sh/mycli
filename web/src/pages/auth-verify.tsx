import { useEffect, useState } from "react";
import { useSearchParams, Navigate } from "react-router";
import { Card } from "../components/ui/card";
import { useAuth } from "../lib/use-auth";

export function AuthVerify() {
  const [searchParams] = useSearchParams();
  const { verify, isAuthenticated } = useAuth();

  const token = searchParams.get("token");

  const [error, setError] = useState(!token ? "Missing verification token" : "");
  const [verifying, setVerifying] = useState(!!token);

  useEffect(() => {
    if (!token) return;

    verify(token)
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Verification failed");
      })
      .finally(() => setVerifying(false));
  }, [token, verify]);

  if (isAuthenticated && !verifying) {
    return <Navigate to="/dashboard" replace />;
  }

  return (
    <div className="max-w-md mx-auto mt-20">
      <Card className="text-center">
        {verifying ? (
          <>
            <div className="animate-spin w-8 h-8 border-2 border-zinc-700 border-t-violet-500 rounded-full mx-auto mb-4" />
            <p className="text-zinc-400">Verifying your email...</p>
          </>
        ) : error ? (
          <>
            <div className="text-red-400 text-lg mb-2">Verification failed</div>
            <p className="text-sm text-zinc-500">{error}</p>
          </>
        ) : null}
      </Card>
    </div>
  );
}
