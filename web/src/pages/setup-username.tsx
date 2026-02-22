import { useState, useCallback } from "react";
import { Navigate } from "react-router";
import { Card } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { useAuth } from "../lib/use-auth";
import { meApi } from "../lib/api";

const usernameRegex = /^[a-z][a-z0-9-]*$/;

export function SetupUsername() {
  const { user, isAuthenticated, isLoading } = useAuth();
  const [username, setUsername] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [checking, setChecking] = useState(false);

  const validate = useCallback((value: string): string => {
    if (value.length < 3) return "Username must be at least 3 characters.";
    if (value.length > 39) return "Username must be at most 39 characters.";
    if (!usernameRegex.test(value)) return "Must start with a letter and contain only lowercase letters, numbers, and hyphens.";
    return "";
  }, []);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[50vh]">
        <div className="text-zinc-500">Loading...</div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />;
  }

  if (user && !user.needs_username) {
    return <Navigate to="/dashboard" replace />;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");

    const validationError = validate(username);
    if (validationError) {
      setError(validationError);
      return;
    }

    setChecking(true);
    try {
      const result = await meApi.checkAvailable(username);
      if (!result.available) {
        setError(result.reason || "Username is not available.");
        setChecking(false);
        return;
      }
    } catch {
      setError("Could not check availability. Try again.");
      setChecking(false);
      return;
    }
    setChecking(false);

    setLoading(true);
    try {
      await meApi.setUsername(username);
      // Force a fresh /me fetch by reloading
      window.location.href = "/dashboard";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to set username.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="max-w-md mx-auto mt-20">
      <Card>
        <h1 className="text-xl font-bold text-zinc-100 mb-2">Choose a username</h1>
        <p className="text-sm text-zinc-400 mb-6">
          Your username will be used in library slugs (e.g.{" "}
          <span className="font-mono text-zinc-300">username/my-library</span>).
          Must be 3-39 characters: lowercase letters, numbers, and hyphens.
        </p>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            type="text"
            placeholder="your-username"
            value={username}
            onChange={(e) => setUsername(e.target.value.toLowerCase())}
            autoFocus
            required
          />
          {error && <p className="text-sm text-red-400">{error}</p>}
          <Button
            type="submit"
            className="w-full"
            disabled={loading || checking}
          >
            {checking ? "Checking..." : loading ? "Setting..." : "Set username"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
