package worker

import (
	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
)

type Repository interface {
	CreatePairing(req workermodel.PairingRequest) workermodel.Pairing
	ApprovePairing(pairingCode string, tenantID string, userID string) error
	GetPairing(pairingID string) (workermodel.Pairing, error)
	DeviceByToken(token string) (workermodel.Device, error)
	ListDevices(opts workermodel.ListDevicesOptions) ([]workermodel.Device, error)
	Heartbeat(deviceID string, req workermodel.HeartbeatRequest) (workermodel.Device, error)
	RevokeDevice(tenantID string, deviceID string) (workermodel.Device, error)
}

type Engine struct {
	repo               Repository
	autoApprovePairing bool
	defaultTenantID    string
	defaultUserID      string
}

type Options struct {
	AutoApprovePairing bool
	DefaultTenantID    string
	DefaultUserID      string
}

func New(repo Repository, options ...Options) *Engine {
	engine := &Engine{repo: repo}
	if len(options) > 0 {
		engine.autoApprovePairing = options[0].AutoApprovePairing
		engine.defaultTenantID = options[0].DefaultTenantID
		engine.defaultUserID = options[0].DefaultUserID
	}
	return engine
}

func (engine *Engine) CreatePairing(req workermodel.PairingRequest) workermodel.Pairing {
	pairing := engine.repo.CreatePairing(req)
	if engine.autoApprovePairing {
		if err := engine.repo.ApprovePairing(pairing.Code, engine.defaultTenantID, engine.defaultUserID); err != nil {
			panic(err)
		}
	}
	return pairing
}

func (engine *Engine) ApprovePairing(pairingCode string, tenantID string, userID string) error {
	return engine.repo.ApprovePairing(pairingCode, tenantID, userID)
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

func (engine *Engine) RevokeDevice(tenantID string, deviceID string) (workermodel.Device, error) {
	return engine.repo.RevokeDevice(tenantID, deviceID)
}
