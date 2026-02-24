package store

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	dbgen "mycli.sh/api/internal/database/generated"
	"mycli.sh/api/internal/model"
)

func tsToTime(ts pgtype.Timestamptz) time.Time {
	return ts.Time
}

func timeToTs(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func toModelUser(u dbgen.User) model.User {
	return model.User{
		ID:        u.ID,
		Email:     u.Email,
		Username:  u.Username,
		CreatedAt: tsToTime(u.CreatedAt),
	}
}

func toModelCommand(c dbgen.Command) model.Command {
	return model.Command{
		ID:          c.ID,
		OwnerUserID: c.OwnerUserID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		Tags:        c.Tags,
		LibraryID:   c.LibraryID,
		CreatedAt:   tsToTime(c.CreatedAt),
		UpdatedAt:   tsToTime(c.UpdatedAt),
		DeletedAt:   c.DeletedAt,
	}
}

func toModelCommandVersion(v dbgen.CommandVersion) model.CommandVersion {
	return model.CommandVersion{
		ID:        v.ID,
		CommandID: v.CommandID,
		Version:   int(v.Version),
		SpecJSON:  v.SpecJson,
		SpecHash:  v.SpecHash,
		Message:   v.Message,
		CreatedBy: v.CreatedBy,
		CreatedAt: tsToTime(v.CreatedAt),
	}
}

func toModelMagicLink(ml dbgen.MagicLink) model.MagicLink {
	return model.MagicLink{
		ID:         ml.ID,
		Email:      ml.Email,
		TokenHash:  ml.TokenHash,
		DeviceCode: ml.DeviceCode,
		OTPHash:    ml.OtpHash,
		ExpiresAt:  tsToTime(ml.ExpiresAt),
		UsedAt:     ml.UsedAt,
		CreatedAt:  tsToTime(ml.CreatedAt),
	}
}

func toModelSessionFromCreate(s dbgen.CreateSessionRow) model.Session {
	return model.Session{
		ID:               s.ID,
		UserID:           s.UserID,
		RefreshTokenHash: s.RefreshTokenHash,
		UserAgent:        s.UserAgent,
		IPAddress:        s.IpAddress,
		DeviceID:         s.DeviceID,
		DeviceName:       s.DeviceName,
		LastUsedAt:       tsToTime(s.LastUsedAt),
		ExpiresAt:        tsToTime(s.ExpiresAt),
		RevokedAt:        s.RevokedAt,
		CreatedAt:        tsToTime(s.CreatedAt),
	}
}

func toModelSessionFromList(s dbgen.ListSessionsByUserRow) model.Session {
	return model.Session{
		ID:               s.ID,
		UserID:           s.UserID,
		RefreshTokenHash: s.RefreshTokenHash,
		UserAgent:        s.UserAgent,
		IPAddress:        s.IpAddress,
		DeviceID:         s.DeviceID,
		DeviceName:       s.DeviceName,
		LastUsedAt:       tsToTime(s.LastUsedAt),
		ExpiresAt:        tsToTime(s.ExpiresAt),
		RevokedAt:        s.RevokedAt,
		CreatedAt:        tsToTime(s.CreatedAt),
	}
}

func toModelSessionFromTokenHash(s dbgen.GetSessionByTokenHashRow) model.Session {
	return model.Session{
		ID:               s.ID,
		UserID:           s.UserID,
		RefreshTokenHash: s.RefreshTokenHash,
		UserAgent:        s.UserAgent,
		IPAddress:        s.IpAddress,
		DeviceID:         s.DeviceID,
		DeviceName:       s.DeviceName,
		LastUsedAt:       tsToTime(s.LastUsedAt),
		ExpiresAt:        tsToTime(s.ExpiresAt),
		RevokedAt:        s.RevokedAt,
		CreatedAt:        tsToTime(s.CreatedAt),
	}
}

func toModelLibrary(l dbgen.Library) model.Library {
	return model.Library{
		ID:            l.ID,
		OwnerID:       l.OwnerID,
		Slug:          l.Slug,
		Name:          l.Name,
		Description:   l.Description,
		GitURL:        l.GitUrl,
		IsPublic:      l.IsPublic,
		InstallCount:  int(l.InstallCount),
		LatestVersion: l.LatestVersion,
		CreatedAt:     tsToTime(l.CreatedAt),
		UpdatedAt:     tsToTime(l.UpdatedAt),
	}
}

func toModelDeviceSession(ds dbgen.DeviceSession) model.DeviceSession {
	return model.DeviceSession{
		ID:          ds.ID,
		DeviceCode:  ds.DeviceCode,
		UserCode:    ds.UserCode,
		Email:       ds.Email,
		ExpiresAt:   tsToTime(ds.ExpiresAt),
		Authorized:  ds.Authorized,
		UserID:      ds.UserID,
		OTPAttempts: int(ds.OtpAttempts),
		CreatedAt:   tsToTime(ds.CreatedAt),
	}
}

func toModelLibraryRelease(r dbgen.LibraryRelease) model.LibraryRelease {
	return model.LibraryRelease{
		ID:           r.ID,
		LibraryID:    r.LibraryID,
		Version:      r.Version,
		Tag:          r.Tag,
		CommitHash:   r.CommitHash,
		CommandCount: int(r.CommandCount),
		ReleasedBy:   r.ReleasedBy,
		ReleasedAt:   tsToTime(r.ReleasedAt),
	}
}
