import type { Metadata } from "next";
import Link from "next/link";
import { ClipboardList, Home, KeyRound } from "lucide-react";
import { AccountNav } from "@/components/AccountNav";
import "./globals.css";

export const metadata: Metadata = {
  title: "Browser Agent 自动化",
  description: "自动化运营与版权监测控制台",
};

const navItems = [
  { href: "/", label: "工作台", icon: Home },
  { href: "/jobs", label: "任务状态", icon: ClipboardList },
  { href: "/worker/pair", label: "连接 Worker", icon: KeyRound },
];

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body>
        <div className="app-shell">
          <header className="topbar">
            <div className="brand-area">
              <span className="brand">Agent</span>
              <h1 className="title">自动化运营控制台</h1>
            </div>
            <nav className="nav-links">
              {navItems.map((item) => {
                const Icon = item.icon;
                return (
                  <Link key={item.href} href={item.href} className="nav-link">
                    <Icon size={16} />
                    <span>{item.label}</span>
                  </Link>
                );
              })}
              <AccountNav />
            </nav>
          </header>
          <main className="content">{children}</main>
        </div>
      </body>
    </html>
  );
}
