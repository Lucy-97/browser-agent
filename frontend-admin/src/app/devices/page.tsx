"use client";

import { useEffect, useState } from "react";
import { Topbar } from "@/components/Topbar";
import { DataSection, EmptyRow, StatusBadge } from "@/components/UI";
import { api, errorMessage } from "@/lib/api";
import { Device } from "@/lib/types";
import { formatDateTime } from "@/lib/utils";
import { Ban, Loader2 } from "lucide-react";

export default function DevicesPage() {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState("");
  const [error, setError] = useState("");

  const fetchDevices = async () => {
    setLoading(true);
    setError("");
    try {
      const res = await api<{ devices: Device[] }>("/admin/worker/devices?limit=50");
      setDevices(res.devices);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void fetchDevices();
  }, []);

  const revokeDevice = async (deviceID: string) => {
    setActionLoading(deviceID);
    setError("");
    try {
      await api<Device>(`/admin/worker/devices/${deviceID}/revoke`, { method: "POST" });
      await fetchDevices();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setActionLoading("");
    }
  };

  return (
    <>
      <Topbar onRefresh={fetchDevices} loading={loading} />
      {error && <div className="error-banner">{error}</div>}
      <section className="workspace">
        <DataSection title="设备管理" count={devices.length}>
          <table>
            <thead>
              <tr>
                <th>设备 ID</th>
                <th>状态</th>
                <th>名称</th>
                <th>平台</th>
                <th>版本</th>
                <th>能力</th>
                <th>最后上线</th>
                <th className="right">操作</th>
              </tr>
            </thead>
            <tbody>
              {devices.map((device) => (
                <tr key={device.id}>
                  <td className="mono" title={device.id}>{device.id}</td>
                  <td><StatusBadge status={device.status} /></td>
                  <td>{device.name}</td>
                  <td>{device.platform}</td>
                  <td>{device.worker_version}</td>
                  <td className="truncate">{(device.capabilities || []).join(", ") || "-"}</td>
                  <td>{formatDateTime(device.last_heartbeat)}</td>
                  <td className="right">
                    <button
                      type="button"
                      className="danger-button"
                      onClick={() => revokeDevice(device.id)}
                      disabled={device.status === "revoked" || actionLoading === device.id}
                      title="撤销设备"
                    >
                      {actionLoading === device.id ? <Loader2 className="spin" size={16} /> : <Ban size={16} />}
                      <span>撤销</span>
                    </button>
                  </td>
                </tr>
              ))}
              {!devices.length && <EmptyRow colSpan={8} />}
            </tbody>
          </table>
        </DataSection>
      </section>
    </>
  );
}
