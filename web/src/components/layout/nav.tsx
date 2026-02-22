import { Link } from "react-router";
import { useAuth } from "../../lib/use-auth";
import { Button } from "../ui/button";
import { Logo } from "../ui/logo";

export function Nav() {
  const { isAuthenticated, user, logout } = useAuth();

  return (
    <nav className="sticky top-0 z-50 bg-zinc-950/80 backdrop-blur-xl border-b border-zinc-800">
      <div className="max-w-6xl mx-auto px-4 h-14 flex items-center justify-between">
        <div className="flex items-center gap-6">
          <Link to="/" className="hover:opacity-80 transition-opacity">
            <Logo className="h-6 w-auto" />
          </Link>
          <div className="hidden sm:flex items-center gap-4">
            <Link to="/" className="text-sm text-zinc-400 hover:text-zinc-100 transition-colors">
              Libraries
            </Link>
            {isAuthenticated && (
              <>
                <Link to="/dashboard" className="text-sm text-zinc-400 hover:text-zinc-100 transition-colors">
                  Dashboard
                </Link>
                <Link to="/sessions" className="text-sm text-zinc-400 hover:text-zinc-100 transition-colors">
                  Sessions
                </Link>
              </>
            )}
          </div>
        </div>
        <div className="flex items-center gap-3">
          {isAuthenticated ? (
            <>
              <span className="text-sm text-zinc-500 hidden sm:inline">
                {user?.email}
              </span>
              <Button variant="ghost" size="sm" onClick={logout}>
                Logout
              </Button>
            </>
          ) : (
            <Link to="/login">
              <Button variant="secondary" size="sm">
                Sign in
              </Button>
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}
