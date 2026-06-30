import type { Metadata } from "next";
import "./globals.css";
import { Navigation } from "@/components/Navigation";

export const metadata: Metadata = {
  title: "Browser Agent Admin",
  description: "自动化控制台"
};

export default function RootLayout({
  children
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body>
        <div className="app-shell">
          <header className="global-topbar">
            <div className="brand-area">
              <span className="brand-mark">B</span>
              <h1 className="title">Browser Agent Admin</h1>
            </div>
            <Navigation />
          </header>
          <main className="content">{children}</main>
        </div>
      </body>
    </html>
  );
}
