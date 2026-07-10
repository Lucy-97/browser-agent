package worker

import (
	workermodel "qiyuan/backend-api/internal/model/worker"
)

type Repository interface {
	CreatePairing(req workermodel.PairingRequest) workermodel.Pairing
	GetPairing(pairingID string) (workermodel.Pairing, error)
	DeviceByToken(token string) (workermodel.Device, error)
	ListDevices(opts workermodel.ListDevicesOptions) ([]workermodel.Device, error)
	Heartbeat(deviceID string, req workermodel.HeartbeatRequest) (workermodel.Device, error)
	RevokeDevice(deviceID string) (workermodel.Device, error)
}

type Engine struct {
	repo Repository
}

func New(repo Repository) *Engine {
	return &Engine{repo: repo}
}

func (engine *Engine) CreatePairing(req workermodel.PairingRequest) workermodel.Pairing {
	return engine.repo.CreatePairing(req)
}

func (engine *Engine) GetPairing(pairingID string) (workermodel.Pairing, error) {
	return engine.repo.GetPairing(pairingID)
}

func (engine *Engine) DeviceByToken(token string) (workermodel.Device, error) {
	return engine.repo.DeviceByToken(token)
}

func (engine *Engine) ListDevices(opts workermodel.ListDevicesOptions) ([]workermodel.Device, error) {
	if opts.Limit <= 0 {
		opts.Limit = 50
	}
	if opts.Limit > 200 {
		opts.Limit = 200
	}
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	return engine.repo.ListDevices(opts)
}

func (engine *Engine) Heartbeat(deviceID string, req workermodel.HeartbeatRequest) (workermodel.Device, error) {
	return engine.repo.Heartbeat(deviceID, req)
}

func (engine *Engine) RevokeDevice(deviceID string) (workermodel.Device, error) {
	return engine.repo.RevokeDevice(deviceID)
}
