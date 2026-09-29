package infrastructure

import (
	d "InkLore/internal/agent/domain"
	pe "InkLore/internal/platform/errors"
	"InkLore/internal/platform/uow"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Repository implements d.GovernanceRepository for PostgreSQL.
type Repository struct {
	manager *uow.Manager
}

// NewRepository creates a new repository instance.
func NewRepository(manager *uow.Manager) (*Repository, error) {
	if manager == nil {
		return nil, errors.New("agent infrastructure: uow manager required")
	}
	return &Repository{manager: manager}, nil
}

func (r *Repository) db(ctx context.Context, write bool) (*gorm.DB, error) {
	var db *gorm.DB
	var err error
	if write {
		db, err = r.manager.Require(ctx)
	} else {
		db, err = r.manager.DB(ctx)
	}
	if err != nil {
		return nil, err
	}
	return db.Session(&gorm.Session{Logger: logger.Discard}), nil
}

func handleNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d.ErrNotFound()
	}
	return err
}

func parseListCursor(query d.ListQuery) (timeStr string, idStr string, hasCursor bool, err error) {
	if query.BeforeTime != "" || query.BeforeID != "" {
		if query.BeforeTime == "" || query.BeforeID == "" {
			return "", "", false, pe.Contract(400, "platform.cursor_invalid", nil)
		}
		return query.BeforeTime, query.BeforeID, true, nil
	}
	return "", "", false, nil
}
