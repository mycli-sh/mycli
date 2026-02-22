import { BrowserRouter, Routes, Route } from "react-router";
import { RootLayout } from "./components/layout/root-layout";
import { AuthGuard } from "./lib/auth";
import { Home } from "./pages/home";
import { LibraryDetailPage } from "./pages/library-detail";
import { CommandDetailPage } from "./pages/command-detail";
import { Login } from "./pages/login";
import { AuthVerify } from "./pages/auth-verify";
import { Dashboard } from "./pages/dashboard";
import { Sessions } from "./pages/sessions";
import { SetupUsername } from "./pages/setup-username";

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<RootLayout />}>
          <Route index element={<Home />} />
          <Route path="libraries/:owner/:slug" element={<LibraryDetailPage />} />
          <Route path="libraries/:owner/:slug/commands/:commandSlug" element={<CommandDetailPage />} />
          <Route path="login" element={<Login />} />
          <Route path="auth/verify" element={<AuthVerify />} />
          <Route path="setup-username" element={<SetupUsername />} />
          <Route
            path="dashboard"
            element={
              <AuthGuard>
                <Dashboard />
              </AuthGuard>
            }
          />
          <Route
            path="sessions"
            element={
              <AuthGuard>
                <Sessions />
              </AuthGuard>
            }
          />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
