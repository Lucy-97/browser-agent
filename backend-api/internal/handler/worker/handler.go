package worker

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	workerengine "qiyuan/backend-api/internal/engine/worker"
	basehandler "qiyuan/backend-api/internal/handler"
	workermodel "qiyuan/backend-api/internal/model/worker"
	workerrepo "qiyuan/backend-api/internal/repository/worker"
)

type Handler struct {
	engine *workerengine.Engine
}

func New(engine *workerengine.Engine) *Handler {
	return &Handler{engine: engine}
}

func (handler *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/worker/devices", handler.listDevices)
	mux.HandleFunc("POST /admin/worker/devices/{device_id}/revoke", handler.revokeDevice)
	mux.HandleFunc("POST /worker/devices/pairing", handler.createPairing)
	mux.HandleFunc("GET /worker/devices/pairing/{pairing_id}", handler.getPairing)
	mux.HandleFunc("POST /worker/devices/{device_id}/heartbeat", handler.heartbeat)
}

func (handler *Handler) AuthenticatedDevice(r *http.Request) (workermodel.Device, bool) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		return workermodel.Device{}, false
	}
	device, err := handler.engine.DeviceByToken(token)
	return device, err == nil
}

func (handler *Handler) createPairing(w http.ResponseWriter, r *http.Request) {
	var req workermodel.PairingRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusCreated, handler.engine.CreatePairing(req))
}

func (handler *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	limit, offset := limitOffset(r)
	devices, err := handler.engine.ListDevices(workermodel.ListDevicesOptions{
		Status: r.URL.Query().Get("status"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		status, code := mapWorkerError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, map[string]any{"devices": devices, "limit": limit, "offset": offset})
}

func (handler *Handler) revokeDevice(w http.ResponseWriter, r *http.Request) {
	device, err := handler.engine.RevokeDevice(r.PathValue("device_id"))
	if err != nil {
		status, code := mapWorkerError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, device)
}

func (handler *Handler) getPairing(w http.ResponseWriter, r *http.Request) {
	pairing, err := handler.engine.GetPairing(r.PathValue("pairing_id"))
	if err != nil {
		status, code := mapWorkerError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, pairing)
}

func (handler *Handler) heartbeat(w http.ResponseWriter, r *http.Request) {
	device, ok := handler.AuthenticatedDevice(r)
	if !ok {
		basehandler.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing or invalid device token", false)
		return
	}
	deviceID := r.PathValue("device_id")
	if device.ID != deviceID {
		basehandler.WriteError(w, http.StatusForbidden, "DEVICE_MISMATCH", "token does not match device", false)
		return
	}
	var req workermodel.HeartbeatRequest
	if err := basehandler.DecodeJSON(r, &req); err != nil {
		basehandler.WriteError(w, http.StatusBadRequest, "INVALID_JSON", err.Error(), false)
		return
	}
	updated, err := handler.engine.Heartbeat(deviceID, req)
	if err != nil {
		status, code := mapWorkerError(err)
		basehandler.WriteError(w, status, code, err.Error(), false)
		return
	}
	basehandler.WriteJSON(w, http.StatusOK, updated)
}

func mapWorkerError(err error) (int, string) {
	switch {
	case errors.Is(err, workerrepo.ErrDeviceNotFound), errors.Is(err, workerrepo.ErrPairingNotFound):
		return http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, workerrepo.ErrDeviceRevoked):
		return http.StatusForbidden, "DEVICE_REVOKED"
	default:
		return http.StatusInternalServerError, "INTERNAL_ERROR"
	}
}

func limitOffset(r *http.Request) (int, int) {
	limit := parseIntDefault(r.URL.Query().Get("limit"), 50)
	offset := parseIntDefault(r.URL.Query().Get("offset"), 0)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
