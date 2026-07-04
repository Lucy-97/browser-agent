package worker

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
)

var (
	ErrDeviceNotFound  = errors.New("device not found")
	ErrPairingNotFound = errors.New("pairing not found")
	ErrDeviceRevoked   = errors.New("device revoked")
)

type MemoryRepository struct {
	mu             sync.Mutex
	pairings       map[string]*workermodel.Pairing
	devices        map[string]*workermodel.Device
	devicesByToken map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		pairings:       map[string]*workermodel.Pairing{},
		devices:        map[string]*workermodel.Device{},
		devicesByToken: map[string]string{},
	}
}

func (repo *MemoryRepository) CreatePairing(req workermodel.PairingRequest) workermodel.Pairing {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	now := time.Now().UTC()
	pairing := workermodel.Pairing{
		ID:                  newID("pair"),
		Code:                shortCode(),
		VerificationURI:     "http://localhost:23001/worker/pair",
		ExpiresAt:           now.Add(10 * time.Minute),
		PollIntervalSeconds: 1,
		Status:              "pending",
		DisplayName:         req.DisplayName,
		Platform:            req.Platform,
		WorkerVersion:       req.WorkerVersion,
	}
	repo.pairings[pairing.ID] = &pairing
	return pairing
}

func (repo *MemoryRepository) GetPairing(pairingID string) (workermodel.Pairing, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	pairing, ok := repo.pairings[pairingID]
	if !ok {
		return workermodel.Pairing{}, ErrPairingNotFound
	}
	if pairing.Status == "pending" {
		deviceID := newID("wdev")
		token := newID("wdt")
		name := pairing.DisplayName
		if name == "" {
			name = fmt.Sprintf("Worker %s", pairing.Code)
		}
		device := workermodel.Device{
			ID:            deviceID,
			Name:          name,
			Platform:      pairing.Platform,
			WorkerVersion: pairing.WorkerVersion,
			Token:         token,
			Status:        "paired",
			Capabilities:  []string{},
		}
		repo.devices[device.ID] = &device
		repo.devicesByToken[token] = device.ID
		pairing.Status = "approved"
		pairing.DeviceID = device.ID
		pairing.Device = &device
		pairing.DeviceToken = token
	}
	return *pairing, nil
}

func (repo *MemoryRepository) DeviceByToken(token string) (workermodel.Device, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	deviceID, ok := repo.devicesByToken[token]
	if !ok {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	device := repo.devices[deviceID]
	if device.Revoked {
		return workermodel.Device{}, ErrDeviceRevoked
	}
	return *device, nil
}

func (repo *MemoryRepository) ListDevices(opts workermodel.ListDevicesOptions) ([]workermodel.Device, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	devices := make([]workermodel.Device, 0, len(repo.devices))
	for _, device := range repo.devices {
		if opts.Status != "" && device.Status != opts.Status {
			continue
		}
		devices = append(devices, *device)
	}
	sort.Slice(devices, func(i, j int) bool {
		left := devices[i].LastHeartbeat
		right := devices[j].LastHeartbeat
		if left == nil {
			return false
		}
		if right == nil {
			return true
		}
		return left.After(*right)
	})
	if opts.Offset >= len(devices) {
		return []workermodel.Device{}, nil
	}
	end := opts.Offset + opts.Limit
	if end > len(devices) {
		end = len(devices)
	}
	return append([]workermodel.Device{}, devices[opts.Offset:end]...), nil
}

func (repo *MemoryRepository) Heartbeat(deviceID string, req workermodel.HeartbeatRequest) (workermodel.Device, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	device, ok := repo.devices[deviceID]
	if !ok {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	if device.Revoked {
		return workermodel.Device{}, ErrDeviceRevoked
	}
	now := time.Now().UTC()
	device.Status = req.Status
	device.WorkerVersion = req.WorkerVersion
	device.Capabilities = append([]string{}, req.Capabilities...)
	device.Metrics = req.Metrics
	device.LastHeartbeat = &now
	return *device, nil
}

func (repo *MemoryRepository) RevokeDevice(deviceID string) (workermodel.Device, error) {
	repo.mu.Lock()
	defer repo.mu.Unlock()

	device, ok := repo.devices[deviceID]
	if !ok {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	device.Status = "revoked"
	device.Revoked = true
	return *device, nil
}

func newID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

func shortCode() string {
	var buf [3]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}
