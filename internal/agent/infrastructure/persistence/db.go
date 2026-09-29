package enginepersistence

import (
	"fmt"

	"gorm.io/gorm"
)

// DB is the repository database handle. Runtime code must pass a *gorm.DB.
type DB struct {
	gorm *gorm.DB
}

func NewDB(conn any) *DB {
	switch db := conn.(type) {
	case *DB:
		return db
	case *gorm.DB:
		return &DB{gorm: db}
	default:
		panic(fmt.Sprintf("repository: unsupported DB handle %T", conn))
	}
}

func (db *DB) GORM() *gorm.DB {
	if db == nil {
		return nil
	}
	return db.gorm
}
