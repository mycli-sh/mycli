import { Outlet } from "react-router";
import { Nav } from "./nav";

export function RootLayout() {
  return (
    <div className="min-h-screen bg-zinc-950">
      <Nav />
      <main className="max-w-6xl mx-auto px-4 py-8">
        <Outlet />
      </main>
    </div>
  );
}
