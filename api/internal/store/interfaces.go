package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"mycli.sh/api/internal/model"
)

// CommandStore is the subset of Store used by CommandHandler.
type CommandStore interface {
	CreateCommand(ctx context.Context, ownerID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	GetCommandByID(ctx context.Context, id uuid.UUID) (*model.Command, error)
	GetCommandByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Command, error)
	ListCommandsByOwner(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error)
	SoftDeleteCommand(ctx context.Context, id uuid.UUID) error
	GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	GetLatestHashByCommand(ctx context.Context, commandID uuid.UUID) (string, error)
	CreateVersion(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error)
	GetVersionByCommandAndVersion(ctx context.Context, commandID uuid.UUID, version int) (*model.CommandVersion, error)
	IsLibraryInstalled(ctx context.Context, userID, libraryID uuid.UUID) bool
}

// CatalogStore is the subset of Store used by CatalogHandler.
type CatalogStore interface {
	ListCommandsByOwner(ctx context.Context, ownerID uuid.UUID, cursor string, limit int, query string) ([]model.Command, string, error)
	GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error)
	ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryCommand, error)
	GetProfileByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Profile, error)
	ListProfileLibraries(ctx context.Context, profileID uuid.UUID) ([]model.Library, error)
	GetDefaultProfile(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error)
}

// MeStore is the subset of Store used by MeHandler.
type MeStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	IsUsernameTaken(ctx context.Context, username string) (bool, error)
	SetUsername(ctx context.Context, userID uuid.UUID, username string) error
	CountCommandsByOwner(ctx context.Context, ownerID uuid.UUID) (int, error)
	GetDefaultProfile(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error)
	ListProfileLibraries(ctx context.Context, profileID uuid.UUID) ([]model.Library, error)
	ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryCommand, error)
	GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error)
}

// SessionStore is the subset of Store used by SessionHandler.
type SessionStore interface {
	ListSessionsByUser(ctx context.Context, userID uuid.UUID) ([]model.Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeAllSessionsExcept(ctx context.Context, userID, exceptID uuid.UUID) (int64, error)
}

// AuthStore is the subset of Store used by AuthHandler.
type AuthStore interface {
	CreateMagicLink(ctx context.Context, email, tokenHash, deviceCode string, otpHash *string, expiresAt time.Time) (*model.MagicLink, error)
	GetMagicLinkByTokenHash(ctx context.Context, tokenHash string) (*model.MagicLink, error)
	GetMagicLinkByOTPHash(ctx context.Context, otpHash string) (*model.MagicLink, error)
	GetMagicLinkByDeviceCode(ctx context.Context, deviceCode string) (*model.MagicLink, error)
	MarkMagicLinkUsed(ctx context.Context, id uuid.UUID) error
	AuthorizeMagicLinkByDeviceCode(ctx context.Context, deviceCode string, userID uuid.UUID) error
	IncrementMagicLinkOTPAttempts(ctx context.Context, id uuid.UUID) (int, error)
	DeleteMagicLinksByDeviceCode(ctx context.Context, deviceCode string) error
	DeleteExpiredMagicLinks(ctx context.Context) error
	ConsumeAuthorizedDeviceCode(ctx context.Context, deviceCode string, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CreateUserWithDefaultProfile(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	CreateSession(ctx context.Context, userID uuid.UUID, refreshTokenHash, userAgent, ipAddress, deviceID, deviceName string, expiresAt time.Time) (*model.Session, error)
	RevokeSessionByDeviceID(ctx context.Context, userID uuid.UUID, deviceID string) error
	RevokeSession(ctx context.Context, id uuid.UUID) error
	GetSessionByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)
	UpdateSessionLastUsed(ctx context.Context, id uuid.UUID) error
	UpdateSessionRefreshTokenHash(ctx context.Context, id uuid.UUID, newHash string) error
	CountOTPAttemptsByDeviceCode(ctx context.Context, deviceCode string) (int, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
}

// TokenStore is the subset of Store used by TokenHandler.
type TokenStore interface {
	CreateAPIToken(ctx context.Context, userID uuid.UUID, name, tokenHash, tokenPrefix string, profileID *uuid.UUID, expiresAt *time.Time) (*model.APIToken, error)
	ListAPITokens(ctx context.Context, userID uuid.UUID) ([]model.APIToken, error)
	RevokeAPIToken(ctx context.Context, id, userID uuid.UUID) error
	GetAPITokenByHash(ctx context.Context, tokenHash string) (*model.APIToken, error)
	UpdateAPITokenLastUsed(ctx context.Context, id uuid.UUID) error
	GetProfileByOwner(ctx context.Context, ownerID, profileID uuid.UUID) (*model.Profile, error)
}

// ProfileStore is the subset of Store used by ProfileHandler.
type ProfileStore interface {
	CreateProfile(ctx context.Context, ownerID uuid.UUID, slug, name, description string) (*model.Profile, error)
	GetProfileByOwnerAndSlug(ctx context.Context, ownerID uuid.UUID, slug string) (*model.Profile, error)
	ListProfilesByOwner(ctx context.Context, ownerID uuid.UUID) ([]model.Profile, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, name, description string) (*model.Profile, error)
	DeleteProfile(ctx context.Context, id uuid.UUID) error
	GetDefaultProfile(ctx context.Context, ownerID uuid.UUID) (*model.Profile, error)
	AddLibraryToProfile(ctx context.Context, profileID, libraryID uuid.UUID) error
	RemoveLibraryFromProfile(ctx context.Context, profileID, libraryID uuid.UUID) error
	ListProfileLibraries(ctx context.Context, profileID uuid.UUID) ([]model.Library, error)
	GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
	GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error)
	ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryCommand, error)
	GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	CountTokensByProfile(ctx context.Context, profileID uuid.UUID) (int, error)
}

// LibraryStore is the subset of Store used by LibraryHandler.
type LibraryStore interface {
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	SearchPublicLibraries(ctx context.Context, query string, limit, offset int) ([]model.Library, int, error)
	GetOwnerName(ctx context.Context, ownerID uuid.UUID) (string, error)
	GetLibraryByOwnerUsernameAndSlug(ctx context.Context, ownerName, slug string) (*model.Library, error)
	GetLibraryBySlug(ctx context.Context, slug string) (*model.Library, error)
	ListCommandsByLibrary(ctx context.Context, libraryID uuid.UUID) ([]LibraryCommand, error)
	IsLibraryInstalled(ctx context.Context, userID, libraryID uuid.UUID) bool
	GetCommandByLibraryAndSlug(ctx context.Context, libraryID uuid.UUID, slug string) (*model.Command, error)
	SoftDeleteCommand(ctx context.Context, id uuid.UUID) error
	CreateCommandForLibrary(ctx context.Context, ownerID, libraryID uuid.UUID, name, slug, description string, tags json.RawMessage) (*model.Command, error)
	UpdateCommandMeta(ctx context.Context, id uuid.UUID, name, description string, tags json.RawMessage) error
	GetLatestHashByCommand(ctx context.Context, commandID uuid.UUID) (string, error)
	GetLatestVersionByCommand(ctx context.Context, commandID uuid.UUID) (*model.CommandVersion, error)
	CreateVersion(ctx context.Context, commandID uuid.UUID, version int, specJSON json.RawMessage, specHash, message string, createdBy uuid.UUID) (*model.CommandVersion, error)
	ListVersionsByCommand(ctx context.Context, commandID uuid.UUID) ([]model.CommandVersion, error)
	CreateOrUpdateLibrary(ctx context.Context, ownerID uuid.UUID, slug, name, description string, gitURL *string, aliases []string) (*model.Library, error)
	LibraryReleaseExists(ctx context.Context, libraryID uuid.UUID, version string) (bool, error)
	CreateLibraryRelease(ctx context.Context, libraryID uuid.UUID, version, tag, commitHash string, commandCount int, releasedBy uuid.UUID) (*model.LibraryRelease, error)
	UpdateLibraryLatestVersion(ctx context.Context, libraryID uuid.UUID, version string) error
	ListLibraryReleases(ctx context.Context, libraryID uuid.UUID) ([]model.LibraryRelease, error)
	GetLibraryRelease(ctx context.Context, libraryID uuid.UUID, version string) (*model.LibraryRelease, error)
	WithTx(ctx context.Context, fn func(LibraryStore) error) error
}
