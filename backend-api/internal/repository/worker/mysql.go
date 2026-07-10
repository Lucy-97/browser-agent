package worker

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	workermodel "github.com/Lucy-97/browser-agent/backend-api/internal/model/worker"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(db *sql.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (repo *MySQLRepository) CreatePairing(req workermodel.PairingRequest) workermodel.Pairing {
	now := time.Now().UTC()
	pairing := workermodel.Pairing{
		ID:                  mysqlNewID("pair"),
		Code:                mysqlShortCode(),
		VerificationURI:     pairingVerificationURI(),
		ExpiresAt:           now.Add(10 * time.Minute),
		PollIntervalSeconds: 1,
		Status:              "pending",
		DisplayName:         req.DisplayName,
		Platform:            req.Platform,
		WorkerVersion:       req.WorkerVersion,
	}
	_, err := repo.db.ExecContext(
		context.Background(),
		`INSERT INTO worker_pairing (
			id, pairing_code_hash, status, requested_platform, requested_worker_version,
			requested_hostname_hash, requested_capabilities_json, expires_at, created_at
		) VALUES (?, ?, 'pending', ?, ?, ?, JSON_ARRAY(), ?, ?)`,
		pairing.ID,
		tokenHash(pairing.Code),
		req.Platform,
		req.WorkerVersion,
		req.HostnameHash,
		pairing.ExpiresAt,
		now,
	)
	if err != nil {
		panic(fmt.Errorf("create worker pairing: %w", err))
	}
	return pairing
}

func (repo *MySQLRepository) GetPairing(pairingID string) (workermodel.Pairing, error) {
	tx, err := repo.db.BeginTx(context.Background(), nil)
	if err != nil {
		return workermodel.Pairing{}, err
	}
	defer tx.Rollback()

	var pairing workermodel.Pairing
	var requestedPlatform sql.NullString
	var requestedVersion sql.NullString
	var deviceID sql.NullString
	var deviceName sql.NullString
	var devicePlatform sql.NullString
	var deviceVersion sql.NullString
	err = tx.QueryRowContext(
		context.Background(),
		`SELECT
			p.id, p.status, p.device_id, p.requested_platform, p.requested_worker_version, p.expires_at,
			d.name, d.platform, d.worker_version
		FROM worker_pairing p
		LEFT JOIN worker_device d ON d.id = p.device_id
		WHERE p.id = ?
		FOR UPDATE`,
		pairingID,
	).Scan(
		&pairing.ID,
		&pairing.Status,
		&deviceID,
		&requestedPlatform,
		&requestedVersion,
		&pairing.ExpiresAt,
		&deviceName,
		&devicePlatform,
		&deviceVersion,
	)
	if err == sql.ErrNoRows {
		return workermodel.Pairing{}, ErrPairingNotFound
	}
	if err != nil {
		return workermodel.Pairing{}, err
	}

	if pairing.Status == "pending" {
		token := mysqlNewID("wdt")
		device := workermodel.Device{
			ID:            mysqlNewID("wdev"),
			Name:          fmt.Sprintf("Worker %s", pairing.ID[len(pairing.ID)-6:]),
			Platform:      requestedPlatform.String,
			WorkerVersion: requestedVersion.String,
			Token:         token,
			Status:        "active",
			Capabilities:  []string{},
		}
		now := time.Now().UTC()
		_, err = tx.ExecContext(
			context.Background(),
			`INSERT INTO worker_device (
				id, name, platform, worker_version, device_token_hash, status,
				capabilities_json, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, 'active', JSON_ARRAY(), ?, ?)`,
			device.ID,
			device.Name,
			device.Platform,
			device.WorkerVersion,
			tokenHash(token),
			now,
			now,
		)
		if err != nil {
			return workermodel.Pairing{}, err
		}
		_, err = tx.ExecContext(
			context.Background(),
			`UPDATE worker_pairing
			 SET status = 'approved', device_id = ?, approved_at = ?
			 WHERE id = ?`,
			device.ID,
			now,
			pairing.ID,
		)
		if err != nil {
			return workermodel.Pairing{}, err
		}
		pairing.Status = "approved"
		pairing.DeviceID = device.ID
		pairing.Device = &device
		pairing.DeviceToken = token
	} else if deviceID.Valid {
		device := workermodel.Device{
			ID:            deviceID.String,
			Name:          deviceName.String,
			Platform:      devicePlatform.String,
			WorkerVersion: deviceVersion.String,
			Status:        "active",
		}
		pairing.DeviceID = device.ID
		pairing.Device = &device
	}

	if err := tx.Commit(); err != nil {
		return workermodel.Pairing{}, err
	}
	return pairing, nil
}

func (repo *MySQLRepository) DeviceByToken(token string) (workermodel.Device, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, name, platform, worker_version, status, capabilities_json, last_seen_at
		 FROM worker_device
		 WHERE device_token_hash = ?`,
		tokenHash(token),
	)
	device, err := scanDevice(row)
	if err != nil {
		return workermodel.Device{}, err
	}
	if device.Status == "revoked" {
		return workermodel.Device{}, ErrDeviceRevoked
	}
	return device, nil
}

func (repo *MySQLRepository) ListDevices(opts workermodel.ListDevicesOptions) ([]workermodel.Device, error) {
	where := []string{"1 = 1"}
	args := make([]any, 0)
	if opts.Status != "" {
		where = append(where, "status = ?")
		args = append(args, opts.Status)
	}
	args = append(args, opts.Limit, opts.Offset)
	rows, err := repo.db.QueryContext(
		context.Background(),
		`SELECT id, name, platform, worker_version, status, capabilities_json, last_seen_at
		 FROM worker_device
		 WHERE `+strings.Join(where, " AND ")+`
		 ORDER BY COALESCE(last_seen_at, updated_at) DESC
		 LIMIT ? OFFSET ?`,
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]workermodel.Device, 0)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (repo *MySQLRepository) Heartbeat(deviceID string, req workermodel.HeartbeatRequest) (workermodel.Device, error) {
	now := time.Now().UTC()
	capabilities := jsonArray(req.Capabilities)
	_, err := repo.db.ExecContext(
		context.Background(),
		`UPDATE worker_device
		 SET worker_version = ?, status = 'active', capabilities_json = ?,
		     last_seen_at = ?, updated_at = ?
		 WHERE id = ? AND status <> 'revoked'`,
		req.WorkerVersion,
		capabilities,
		now,
		now,
		deviceID,
	)
	if err != nil {
		return workermodel.Device{}, err
	}
	var device workermodel.Device
	var lastSeen sql.NullTime
	err = repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, name, platform, worker_version, status, last_seen_at
		 FROM worker_device
		 WHERE id = ?`,
		deviceID,
	).Scan(&device.ID, &device.Name, &device.Platform, &device.WorkerVersion, &device.Status, &lastSeen)
	if err == sql.ErrNoRows {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	if err != nil {
		return workermodel.Device{}, err
	}
	if device.Status == "revoked" {
		return workermodel.Device{}, ErrDeviceRevoked
	}
	if lastSeen.Valid {
		device.LastHeartbeat = &lastSeen.Time
	}
	device.Capabilities = append([]string{}, req.Capabilities...)
	device.Metrics = req.Metrics
	return device, nil
}

func (repo *MySQLRepository) RevokeDevice(deviceID string) (workermodel.Device, error) {
	now := time.Now().UTC()
	result, err := repo.db.ExecContext(
		context.Background(),
		`UPDATE worker_device
		 SET status = 'revoked', revoked_at = ?, updated_at = ?
		 WHERE id = ?`,
		now,
		now,
		deviceID,
	)
	if err != nil {
		return workermodel.Device{}, err
	}
	if affected, err := result.RowsAffected(); err == nil && affected == 0 {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	device, err := repo.deviceByID(deviceID)
	if err != nil {
		return workermodel.Device{}, err
	}
	device.Revoked = true
	return device, nil
}

func (repo *MySQLRepository) deviceByID(deviceID string) (workermodel.Device, error) {
	row := repo.db.QueryRowContext(
		context.Background(),
		`SELECT id, name, platform, worker_version, status, capabilities_json, last_seen_at
		 FROM worker_device
		 WHERE id = ?`,
		deviceID,
	)
	return scanDevice(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(row scanner) (workermodel.Device, error) {
	var device workermodel.Device
	var capabilitiesRaw sql.NullString
	var lastSeen sql.NullTime
	err := row.Scan(
		&device.ID,
		&device.Name,
		&device.Platform,
		&device.WorkerVersion,
		&device.Status,
		&capabilitiesRaw,
		&lastSeen,
	)
	if err == sql.ErrNoRows {
		return workermodel.Device{}, ErrDeviceNotFound
	}
	if err != nil {
		return workermodel.Device{}, err
	}
	if capabilitiesRaw.Valid {
		_ = json.Unmarshal([]byte(capabilitiesRaw.String), &device.Capabilities)
	}
	if lastSeen.Valid {
		device.LastHeartbeat = &lastSeen.Time
	}
	device.Revoked = device.Status == "revoked"
	return device, nil
}

func tokenHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func mysqlNewID(prefix string) string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

func mysqlShortCode() string {
	var buf [3]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}

func jsonArray(values []string) string {
	raw, err := json.Marshal(values)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
