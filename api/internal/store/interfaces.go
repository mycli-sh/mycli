package store

import (
	"context"
	"encoding/json"
	"time"

	"mycli.sh/api/internal/model"
)

// CommandStore is the subset of Store used by CommandHandler.
type CommandStore interface {
	CreateCommand(ctx context.Context, ownerID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	GetCommandByID(ctx context.Context, id string) (*model.Command, error)
	GetCommandByOwnerAndSlug(ctx context.Context, ownerID, slug string) (*model.Command, error)
	ListCommandsByOwner(ctx context.Context, ownerID, cursor string, limit int, query string) ([]model.Command, string, error)
	SoftDeleteCommand(ctx context.Context, id string) error
	GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error)
	GetLatestHashByCommand(ctx context.Context, commandID string) (string, error)
	CreateVersion(ctx context.Context, commandID string, version int, specJSON json.RawMessage, specHash, message, createdBy string) (*model.CommandVersion, error)
	GetVersionByCommandAndVersion(ctx context.Context, commandID string, version int) (*model.CommandVersion, error)
	IsLibraryInstalled(ctx context.Context, userID, libraryID string) bool
}

// CatalogStore is the subset of Store used by CatalogHandler.
type CatalogStore interface {
	ListCommandsByOwner(ctx context.Context, ownerID, cursor string, limit int, query string) ([]model.Command, string, error)
	GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error)
	GetInstalledLibraries(ctx context.Context, userID string) ([]model.Library, error)
	GetOwnerName(ctx context.Context, ownerID string) (string, error)
	ListCommandsByLibrary(ctx context.Context, libraryID string) ([]LibraryCommand, error)
}

// MeStore is the subset of Store used by MeHandler.
type MeStore interface {
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	IsUsernameTaken(ctx context.Context, username string) (bool, error)
	SetUsername(ctx context.Context, userID, username string) error
	CountCommandsByOwner(ctx context.Context, ownerID string) (int, error)
	GetInstalledLibraries(ctx context.Context, userID string) ([]model.Library, error)
	ListCommandsByLibrary(ctx context.Context, libraryID string) ([]LibraryCommand, error)
	GetOwnerName(ctx context.Context, ownerID string) (string, error)
}

// SessionStore is the subset of Store used by SessionHandler.
type SessionStore interface {
	ListSessionsByUser(ctx context.Context, userID string) ([]model.Session, error)
	RevokeSession(ctx context.Context, id string) error
	RevokeAllSessionsExcept(ctx context.Context, userID, exceptID string) (int64, error)
}

// AuthStore is the subset of Store used by AuthHandler.
type AuthStore interface {
	CreateMagicLink(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error)
	GetMagicLinkByTokenHash(ctx context.Context, tokenHash string) (*model.MagicLink, error)
	GetMagicLinkByOTPHash(ctx context.Context, otpHash string) (*model.MagicLink, error)
	MarkMagicLinkUsed(ctx context.Context, id string) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	CreateSession(ctx context.Context, userID, refreshTokenHash, userAgent, ipAddress string, expiresAt time.Time) (*model.Session, error)
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	UpdateSessionLastUsed(ctx context.Context, id string) error
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
	InstallLibrary(ctx context.Context, userID, libraryID string) error
	CreateDeviceSession(ctx context.Context, deviceCode, userCode, email string, expiresAt time.Time) error
	GetDeviceSessionByCode(ctx context.Context, deviceCode string) (*model.DeviceSession, error)
	GetDeviceSessionByUserCode(ctx context.Context, userCode string) (*model.DeviceSession, error)
	AuthorizeDeviceSession(ctx context.Context, deviceCode, userID string) error
	IncrementDeviceOTPAttempts(ctx context.Context, deviceCode string) (int, error)
	ResetDeviceOTPAndExtend(ctx context.Context, deviceCode string, expiresAt time.Time) error
	DeleteDeviceSession(ctx context.Context, deviceCode string) error
	DeleteExpiredDeviceSessions(ctx context.Context) error
	WithTx(ctx context.Context, fn func(AuthStore) error) error
}

// LibraryStore is the subset of Store used by LibraryHandler.
type LibraryStore interface {
	GetUserByID(ctx context.Context, id string) (*model.User, error)
	SearchPublicLibraries(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error)
	GetOwnerName(ctx context.Context, ownerID string) (string, error)
	GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
	ListCommandsByLibrary(ctx context.Context, libraryID string) ([]LibraryCommand, error)
	IsLibraryInstalled(ctx context.Context, userID, libraryID string) bool
	GetCommandByLibraryAndSlug(ctx context.Context, libraryID, slug string) (*model.Command, error)
	CreateCommandForLibrary(ctx context.Context, ownerID, libraryID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	UpdateCommandMeta(ctx context.Context, id, name, description string, tags json.RawMessage) error
	GetLatestHashByCommand(ctx context.Context, commandID string) (string, error)
	GetLatestVersionByCommand(ctx context.Context, commandID string) (*model.CommandVersion, error)
	CreateVersion(ctx context.Context, commandID string, version int, specJSON json.RawMessage, specHash, message, createdBy string) (*model.CommandVersion, error)
	ListVersionsByCommand(ctx context.Context, commandID string) ([]model.CommandVersion, error)
	CreateOrUpdateLibrary(ctx context.Context, ownerID, slug, name, description string, gitURL *string) (*model.Library, error)
	LibraryReleaseExists(ctx context.Context, libraryID, version string) (bool, error)
	CreateLibraryRelease(ctx context.Context, libraryID, version, tag, commitHash string, commandCount int, releasedBy string) (*model.LibraryRelease, error)
	UpdateLibraryLatestVersion(ctx context.Context, libraryID, version string) error
	InstallLibrary(ctx context.Context, userID, libraryID string) error
	UninstallLibrary(ctx context.Context, userID, libraryID string) error
	ListLibraryReleases(ctx context.Context, libraryID string) ([]model.LibraryRelease, error)
	GetLibraryRelease(ctx context.Context, libraryID, version string) (*model.LibraryRelease, error)
}

// WebAuthStore is the subset of Store used by WebAuthHandler.
type WebAuthStore interface {
	CreateMagicLink(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error)
	GetMagicLinkByTokenHash(ctx context.Context, tokenHash string) (*model.MagicLink, error)
	MarkMagicLinkUsed(ctx context.Context, id string) error
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUser(ctx context.Context, email string) (*model.User, error)
	CreateSession(ctx context.Context, userID, refreshTokenHash, userAgent, ipAddress string, expiresAt time.Time) (*model.Session, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
	InstallLibrary(ctx context.Context, userID, libraryID string) error
}
