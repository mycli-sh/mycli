package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "mycli.sh/api/internal/database/generated"
	"mycli.sh/api/internal/model"
)

// LibraryCommand is a lightweight struct for library command listings.
type LibraryCommand struct {
	CommandID   string    `json:"command_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Store provides unified access to all database operations.
type Store struct {
	pool *pgxpool.Pool
	db   dbgen.DBTX     // pool or tx — used by dynamic queries
	q    *dbgen.Queries // generated type-safe queries
}

// New creates a new Store backed by the given connection pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, db: pool, q: dbgen.New(pool)}
}

// withTx runs fn inside a transaction. If fn returns an error the transaction
// is rolled back; otherwise it is committed.
func (s *Store) withTx(ctx context.Context, fn func(tx *Store) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txStore := &Store{
		pool: s.pool,
		db:   tx,
		q:    s.q.WithTx(tx),
	}

	if err := fn(txStore); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

func (s *Store) CreateUser(ctx context.Context, email string) (*model.User, error) {
	u, err := s.q.CreateUser(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	m := toModelUser(u)
	return &m, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u, err := s.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	m := toModelUser(u)
	return &m, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	m := toModelUser(u)
	return &m, nil
}

func (s *Store) SetUsername(ctx context.Context, userID, username string) error {
	rows, err := s.q.SetUsername(ctx, dbgen.SetUsernameParams{
		ID:       userID,
		Username: &username,
	})
	if err != nil {
		return fmt.Errorf("set username: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	u, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	m := toModelUser(u)
	return &m, nil
}

func (s *Store) IsUsernameTaken(ctx context.Context, username string) (bool, error) {
	taken, err := s.q.IsUsernameTaken(ctx, username)
	if err != nil {
		return false, fmt.Errorf("check username taken: %w", err)
	}
	return taken, nil
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (s *Store) CreateCommand(ctx context.Context, ownerID, name, slug, description string, tags json.RawMessage) (*model.Command, error) {
	if tags == nil {
		tags = json.RawMessage(`[]`)
	}
	c, err := s.q.CreateCommand(ctx, dbgen.CreateCommandParams{
		OwnerUserID: ownerID,
		Name:        name,
		Slug:        slug,
		Description: description,
		Tags:        tags,
	})
	if err != nil {
		return nil, fmt.Errorf("create command: %w", err)
	}
	m := toModelCommand(c)
	return &m, nil
}

func (s *Store) GetCommandByID(ctx context.Context, id string) (*model.Command, error) {
	c, err := s.q.GetCommandByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get command by id: %w", err)
	}
	m := toModelCommand(c)
	return &m, nil
}

func (s *Store) GetCommandByOwnerAndSlug(ctx context.Context, ownerID, slug string) (*model.Command, error) {
	c, err := s.q.GetCommandByOwnerAndSlug(ctx, dbgen.GetCommandByOwnerAndSlugParams{
		OwnerUserID: ownerID,
		Slug:        slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get command by owner and slug: %w", err)
	}
	m := toModelCommand(c)
	return &m, nil
}

func (s *Store) GetCommandByLibraryAndSlug(ctx context.Context, libraryID, slug string) (*model.Command, error) {
	c, err := s.q.GetCommandByLibraryAndSlug(ctx, dbgen.GetCommandByLibraryAndSlugParams{
		LibraryID: &libraryID,
		Slug:      slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get command by library and slug: %w", err)
	}
	m := toModelCommand(c)
	return &m, nil
}

func (s *Store) CreateCommandForLibrary(ctx context.Context, ownerID, libraryID, name, slug, description string, tags json.RawMessage) (*model.Command, error) {
	if tags == nil {
		tags = json.RawMessage(`[]`)
	}
	c, err := s.q.CreateCommandForLibrary(ctx, dbgen.CreateCommandForLibraryParams{
		OwnerUserID: ownerID,
		LibraryID:   &libraryID,
		Name:        name,
		Slug:        slug,
		Description: description,
		Tags:        tags,
	})
	if err != nil {
		return nil, fmt.Errorf("create command for library: %w", err)
	}
	m := toModelCommand(c)
	return &m, nil
}

func (s *Store) CountCommandsByOwner(ctx context.Context, ownerID string) (int, error) {
	count, err := s.q.CountCommandsByOwner(ctx, ownerID)
	if err != nil {
		return 0, fmt.Errorf("count commands by owner: %w", err)
	}
	return int(count), nil
}

func (s *Store) UpdateCommandMeta(ctx context.Context, id, name, description string, tags json.RawMessage) error {
	return s.q.UpdateCommandMeta(ctx, dbgen.UpdateCommandMetaParams{
		ID:          id,
		Name:        name,
		Description: description,
		Tags:        tags,
	})
}

func (s *Store) SoftDeleteCommand(ctx context.Context, id string) error {
	rows, err := s.q.SoftDeleteCommand(ctx, id)
	if err != nil {
		return fmt.Errorf("soft delete command: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Command Versions
// ---------------------------------------------------------------------------

func (s *Store) CreateVersion(ctx context.Context, commandID string, version int, specJSON json.RawMessage, specHash, message, createdBy string) (*model.CommandVersion, error) {
	v, err := s.q.CreateVersion(ctx, dbgen.CreateVersionParams{
		CommandID: commandID,
		Version:   int32(version),
		SpecJson:  specJSON,
		SpecHash:  specHash,
		Message:   message,
		CreatedBy: createdBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create command version: %w", err)
	}
	m := toModelCommandVersion(v)
	return &m, nil
}

func (s *Store) GetVersionByCommandAndVersion(ctx context.Context, commandID string, version int) (*model.CommandVersion, error) {
	v, err := s.q.GetVersionByCommandAndVersion(ctx, dbgen.GetVersionByCommandAndVersionParams{
		CommandID: commandID,
		Version:   int32(version),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get version by command and version: %w", err)
	}
	m := toModelCommandVersion(v)
	return &m, nil
}

func (s *Store) GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error) {
	v, err := s.q.GetLatestVersionByCommand(ctx, commandID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get latest version by command: %w", err)
	}
	m := toModelCommandVersion(v)
	return &m, nil
}

func (s *Store) ListVersionsByCommand(ctx context.Context, commandID string) ([]model.CommandVersion, error) {
	rows, err := s.q.ListVersionsByCommand(ctx, commandID)
	if err != nil {
		return nil, fmt.Errorf("list versions by command: %w", err)
	}
	versions := make([]model.CommandVersion, len(rows))
	for i, r := range rows {
		versions[i] = toModelCommandVersion(r)
	}
	return versions, nil
}

func (s *Store) GetLatestHashByCommand(ctx context.Context, commandID string) (string, error) {
	hash, err := s.q.GetLatestHashByCommand(ctx, commandID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get latest hash by command: %w", err)
	}
	return hash, nil
}

// ---------------------------------------------------------------------------
// Magic Links
// ---------------------------------------------------------------------------

func (s *Store) CreateMagicLink(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error) {
	ml, err := s.q.CreateMagicLink(ctx, dbgen.CreateMagicLinkParams{
		Email:      email,
		TokenHash:  tokenHash,
		DeviceCode: deviceCode,
		OtpHash:    otpHash,
		ExpiresAt:  timeToTs(expiresAt),
	})
	if err != nil {
		return nil, fmt.Errorf("create magic link: %w", err)
	}
	m := toModelMagicLink(ml)
	return &m, nil
}

func (s *Store) GetMagicLinkByTokenHash(ctx context.Context, tokenHash string) (*model.MagicLink, error) {
	ml, err := s.q.GetMagicLinkByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get magic link by token hash: %w", err)
	}
	m := toModelMagicLink(ml)
	return &m, nil
}

func (s *Store) GetMagicLinkByOTPHash(ctx context.Context, otpHash string) (*model.MagicLink, error) {
	ml, err := s.q.GetMagicLinkByOTPHash(ctx, &otpHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get magic link by otp hash: %w", err)
	}
	m := toModelMagicLink(ml)
	return &m, nil
}

func (s *Store) MarkMagicLinkUsed(ctx context.Context, id string) error {
	rows, err := s.q.MarkMagicLinkUsed(ctx, id)
	if err != nil {
		return fmt.Errorf("mark magic link used: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) GetMagicLinkByDeviceCode(ctx context.Context, deviceCode string) (*model.MagicLink, error) {
	ml, err := s.q.GetMagicLinkByDeviceCode(ctx, deviceCode)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get magic link by device code: %w", err)
	}
	m := toModelMagicLink(ml)
	return &m, nil
}

func (s *Store) AuthorizeMagicLinkByDeviceCode(ctx context.Context, deviceCode, userID string) error {
	rows, err := s.q.AuthorizeMagicLinkByDeviceCode(ctx, dbgen.AuthorizeMagicLinkByDeviceCodeParams{
		DeviceCode: deviceCode,
		UserID:     &userID,
	})
	if err != nil {
		return fmt.Errorf("authorize magic link: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) IncrementMagicLinkOTPAttempts(ctx context.Context, id string) (int, error) {
	attempts, err := s.q.IncrementMagicLinkOTPAttempts(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("increment magic link otp attempts: %w", err)
	}
	return int(attempts), nil
}

func (s *Store) DeleteMagicLinksByDeviceCode(ctx context.Context, deviceCode string) error {
	return s.q.DeleteMagicLinksByDeviceCode(ctx, deviceCode)
}

func (s *Store) DeleteExpiredMagicLinks(ctx context.Context) error {
	return s.q.DeleteExpiredMagicLinks(ctx)
}

// ConsumeAuthorizedDeviceCode atomically deletes all magic links for the given
// device code, revokes any existing session for the device, and creates a new
// session — all within a single transaction.
func (s *Store) ConsumeAuthorizedDeviceCode(ctx context.Context, deviceCode, userID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error) {
	var session *model.Session
	err := s.withTx(ctx, func(tx *Store) error {
		if err := tx.q.DeleteMagicLinksByDeviceCode(ctx, deviceCode); err != nil {
			return fmt.Errorf("delete magic links: %w", err)
		}
		if deviceID != "" {
			_ = tx.q.RevokeSessionByDeviceID(ctx, dbgen.RevokeSessionByDeviceIDParams{
				UserID:   userID,
				DeviceID: deviceID,
			})
		}
		sess, err := tx.q.CreateSession(ctx, dbgen.CreateSessionParams{
			UserID:           userID,
			RefreshTokenHash: refreshTokenHash,
			UserAgent:        userAgent,
			IpAddress:        ipAddress,
			DeviceID:         deviceID,
			DeviceName:       deviceName,
			ExpiresAt:        timeToTs(expiresAt),
		})
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		m := toModelSessionFromCreate(sess)
		session = &m
		return nil
	})
	if err != nil {
		return nil, err
	}
	return session, nil
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

func (s *Store) CreateSession(ctx context.Context, userID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error) {
	sess, err := s.q.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:           userID,
		RefreshTokenHash: refreshTokenHash,
		UserAgent:        userAgent,
		IpAddress:        ipAddress,
		DeviceID:         deviceID,
		DeviceName:       deviceName,
		ExpiresAt:        timeToTs(expiresAt),
	})
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	m := toModelSessionFromCreate(sess)
	return &m, nil
}

func (s *Store) ListSessionsByUser(ctx context.Context, userID string) ([]model.Session, error) {
	rows, err := s.q.ListSessionsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	sessions := make([]model.Session, len(rows))
	for i, r := range rows {
		sessions[i] = toModelSessionFromList(r)
	}
	return sessions, nil
}

func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error) {
	sess, err := s.q.GetSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get session by token hash: %w", err)
	}
	m := toModelSessionFromTokenHash(sess)
	return &m, nil
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {
	rows, err := s.q.RevokeSession(ctx, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) RevokeAllSessionsExcept(ctx context.Context, userID, exceptID string) (int64, error) {
	count, err := s.q.RevokeAllSessionsExcept(ctx, dbgen.RevokeAllSessionsExceptParams{
		UserID: userID,
		ID:     exceptID,
	})
	if err != nil {
		return 0, fmt.Errorf("revoke all sessions: %w", err)
	}
	return count, nil
}

func (s *Store) RevokeSessionByDeviceID(ctx context.Context, userID, deviceID string) error {
	return s.q.RevokeSessionByDeviceID(ctx, dbgen.RevokeSessionByDeviceIDParams{
		UserID:   userID,
		DeviceID: deviceID,
	})
}

func (s *Store) UpdateSessionLastUsed(ctx context.Context, id string) error {
	if err := s.q.UpdateSessionLastUsed(ctx, id); err != nil {
		return fmt.Errorf("update session last used: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Libraries
// ---------------------------------------------------------------------------

func (s *Store) GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error) {
	lib, err := s.q.GetLibraryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get library by slug: %w", err)
	}
	m := toModelLibrary(lib)
	return &m, nil
}

func (s *Store) GetLibraryByOwnerSlug(ctx context.Context, ownerID, slug string) (*model.Library, error) {
	lib, err := s.q.GetLibraryByOwnerSlug(ctx, dbgen.GetLibraryByOwnerSlugParams{
		OwnerID: &ownerID,
		Slug:    slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get library by owner/slug: %w", err)
	}
	m := toModelLibrary(lib)
	return &m, nil
}

func (s *Store) GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error) {
	lib, err := s.q.GetLibraryByOwnerUsernameAndSlug(ctx, dbgen.GetLibraryByOwnerUsernameAndSlugParams{
		Username: &ownerName,
		Slug:     slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get library by owner username/slug: %w", err)
	}
	m := toModelLibrary(lib)
	return &m, nil
}

func (s *Store) ListLibraries(ctx context.Context) ([]model.Library, error) {
	rows, err := s.q.ListLibraries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	libs := make([]model.Library, len(rows))
	for i, r := range rows {
		libs[i] = toModelLibrary(r)
	}
	return libs, nil
}

func (s *Store) CreateOrUpdateLibrary(ctx context.Context, ownerID, slug, name, description string, gitURL *string) (*model.Library, error) {
	lib, err := s.q.CreateOrUpdateLibrary(ctx, dbgen.CreateOrUpdateLibraryParams{
		OwnerID:     &ownerID,
		Slug:        slug,
		Name:        name,
		Description: description,
		GitUrl:      gitURL,
	})
	if err != nil {
		return nil, fmt.Errorf("create or update library: %w", err)
	}
	m := toModelLibrary(lib)
	return &m, nil
}

// InstallLibrary records that a user has installed a library, atomically
// incrementing the install count inside a transaction.
func (s *Store) InstallLibrary(ctx context.Context, userID, libraryID string) error {
	return s.withTx(ctx, func(tx *Store) error {
		if err := tx.q.InstallLibrary(ctx, dbgen.InstallLibraryParams{
			UserID:    userID,
			LibraryID: libraryID,
		}); err != nil {
			return fmt.Errorf("install library: %w", err)
		}
		if err := tx.q.IncrementInstallCount(ctx, libraryID); err != nil {
			return fmt.Errorf("increment install count: %w", err)
		}
		return nil
	})
}

// UninstallLibrary removes a user's installation and atomically decrements
// the install count inside a transaction.
func (s *Store) UninstallLibrary(ctx context.Context, userID, libraryID string) error {
	return s.withTx(ctx, func(tx *Store) error {
		rows, err := tx.q.UninstallLibrary(ctx, dbgen.UninstallLibraryParams{
			UserID:    userID,
			LibraryID: libraryID,
		})
		if err != nil {
			return fmt.Errorf("uninstall library: %w", err)
		}
		if rows > 0 {
			if err := tx.q.DecrementInstallCount(ctx, libraryID); err != nil {
				return fmt.Errorf("decrement install count: %w", err)
			}
		}
		return nil
	})
}

func (s *Store) GetInstalledLibraries(ctx context.Context, userID string) ([]model.Library, error) {
	rows, err := s.q.GetInstalledLibraries(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get installed libraries: %w", err)
	}
	libs := make([]model.Library, len(rows))
	for i, r := range rows {
		libs[i] = toModelLibrary(r)
	}
	return libs, nil
}

func (s *Store) IsLibraryInstalled(ctx context.Context, userID, libraryID string) bool {
	exists, err := s.q.IsLibraryInstalled(ctx, dbgen.IsLibraryInstalledParams{
		UserID:    userID,
		LibraryID: libraryID,
	})
	return err == nil && exists
}

func (s *Store) ListCommandsByLibrary(ctx context.Context, libraryID string) ([]LibraryCommand, error) {
	rows, err := s.q.ListCommandsByLibrary(ctx, &libraryID)
	if err != nil {
		return nil, fmt.Errorf("list commands by library: %w", err)
	}
	cmds := make([]LibraryCommand, len(rows))
	for i, r := range rows {
		cmds[i] = LibraryCommand{
			CommandID:   r.ID,
			Slug:        r.Slug,
			Name:        r.Name,
			Description: r.Description,
			UpdatedAt:   tsToTime(r.UpdatedAt),
		}
	}
	return cmds, nil
}

func (s *Store) GetOwnerName(ctx context.Context, ownerID string) (string, error) {
	username, err := s.q.GetOwnerName(ctx, ownerID)
	if err != nil {
		return "", fmt.Errorf("get owner name: %w", err)
	}
	if username != nil {
		return *username, nil
	}
	return "", nil
}

func (s *Store) UpdateLibraryLatestVersion(ctx context.Context, libraryID, version string) error {
	return s.q.UpdateLibraryLatestVersion(ctx, dbgen.UpdateLibraryLatestVersionParams{
		LatestVersion: &version,
		ID:            libraryID,
	})
}

// ---------------------------------------------------------------------------
// Library Releases
// ---------------------------------------------------------------------------

func (s *Store) CreateLibraryRelease(ctx context.Context, libraryID, version, tag, commitHash string, commandCount int, releasedBy string) (*model.LibraryRelease, error) {
	r, err := s.q.CreateLibraryRelease(ctx, dbgen.CreateLibraryReleaseParams{
		LibraryID:    libraryID,
		Version:      version,
		Tag:          tag,
		CommitHash:   commitHash,
		CommandCount: int32(commandCount),
		ReleasedBy:   releasedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("create library release: %w", err)
	}
	m := toModelLibraryRelease(r)
	return &m, nil
}

func (s *Store) GetLibraryRelease(ctx context.Context, libraryID, version string) (*model.LibraryRelease, error) {
	r, err := s.q.GetLibraryRelease(ctx, dbgen.GetLibraryReleaseParams{
		LibraryID: libraryID,
		Version:   version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get library release: %w", err)
	}
	m := toModelLibraryRelease(r)
	return &m, nil
}

func (s *Store) ListLibraryReleases(ctx context.Context, libraryID string) ([]model.LibraryRelease, error) {
	rows, err := s.q.ListLibraryReleases(ctx, libraryID)
	if err != nil {
		return nil, fmt.Errorf("list library releases: %w", err)
	}
	releases := make([]model.LibraryRelease, len(rows))
	for i, r := range rows {
		releases[i] = toModelLibraryRelease(r)
	}
	return releases, nil
}

func (s *Store) LibraryReleaseExists(ctx context.Context, libraryID, version string) (bool, error) {
	exists, err := s.q.LibraryReleaseExists(ctx, dbgen.LibraryReleaseExistsParams{
		LibraryID: libraryID,
		Version:   version,
	})
	if err != nil {
		return false, fmt.Errorf("check library release exists: %w", err)
	}
	return exists, nil
}
