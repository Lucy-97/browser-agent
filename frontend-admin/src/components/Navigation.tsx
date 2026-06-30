"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { ClipboardList, FileText, Laptop, Network, ShieldCheck, SquareActivity } from "lucide-react";

const navItems = [
  { key: "jobs", label: "任务", icon: ClipboardList },
  { key: "runs", label: "执行", icon: SquareActivity },
  { key: "devices", label: "设备", icon: Laptop },
  { key: "manual", label: "人工干预", icon: ShieldCheck }
];

export function Navigation() {
  const pathname = usePathname();

  return (
    <nav className="nav-links">
      {navItems.map((item) => {
        const Icon = item.icon;
        const isActive = pathname === `/${item.key}`;
        return (
          <Link key={item.key} href={`/${item.key}`} className={`nav-link ${isActive ? "active" : ""}`}>
            <Icon size={16} />
            <span>{item.label}</span>
          </Link>
        );
      })}
    </nav>
  );
}
