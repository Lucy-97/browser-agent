"use client";

import { ClipboardList, FileText, Laptop, Network, ShieldCheck, SquareActivity } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

const tabs = [
  { key: "jobs", label: "任务", icon: ClipboardList },
  { key: "runs", label: "执行", icon: SquareActivity },
  { key: "devices", label: "设备", icon: Laptop },
  { key: "manual", label: "人工干预", icon: ShieldCheck },
];

export function Sidebar() {
  const pathname = usePathname();

  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">B</div>
        <div>
          <div className="brand-title">Browser Agent</div>
          <div className="brand-subtitle">自动化监测控制台</div>
        </div>
      </div>
      <nav className="nav-list">
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const isActive = pathname === `/${tab.key}`;
          return (
            <Link
              key={tab.key}
              href={`/${tab.key}`}
              className={isActive ? "nav-item active" : "nav-item"}
            >
              <Icon size={18} />
              <span>{tab.label}</span>
            </Link>
          );
        })}
      </nav>
    </aside>
  );
}
