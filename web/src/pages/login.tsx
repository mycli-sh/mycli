import { useState } from "react";
import { Navigate } from "react-router";
import { Card } from "../components/ui/card";
import { Input } from "../components/ui/input";
import { Button } from "../components/ui/button";
import { useAuth } from "../lib/use-auth";

export function Login() {
  const { isAuthenticated, login } = useAuth();
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  if (isAuthenticated) {
    return <Navigate to="/dashboard" replace />;
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await login(email);
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send login email");
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
    return (
      <div className="max-w-md mx-auto mt-20">
        <Card className="text-center">
          <div className="text-4xl mb-4">
            <svg className="w-12 h-12 mx-auto text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 01-2.25 2.25h-15a2.25 2.25 0 01-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0019.5 4.5h-15a2.25 2.25 0 00-2.25 2.25m19.5 0v.243a2.25 2.25 0 01-1.07 1.916l-7.5 4.615a2.25 2.25 0 01-2.36 0L3.32 8.91a2.25 2.25 0 01-1.07-1.916V6.75" />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-zinc-100 mb-2">
            Check your email
          </h1>
          <p className="text-zinc-400 text-sm">
            We sent a verification link to{" "}
            <span className="font-mono text-zinc-200">{email}</span>.
            Click the link to sign in.
          </p>
        </Card>
      </div>
    );
  }

  return (
    <div className="max-w-md mx-auto mt-20">
      <Card>
        <h1 className="text-xl font-bold text-zinc-100 mb-2">Sign in</h1>
        <p className="text-sm text-zinc-400 mb-6">
          Enter your email and we'll send you a magic link.
        </p>
        <form onSubmit={handleSubmit} className="space-y-4">
          <Input
            type="email"
            placeholder="you@example.com"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          {error && (
            <p className="text-sm text-red-400">{error}</p>
          )}
          <Button type="submit" className="w-full" disabled={loading}>
            {loading ? "Sending..." : "Send magic link"}
          </Button>
        </form>
      </Card>
    </div>
  );
}
