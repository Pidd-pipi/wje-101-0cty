package repository

import "gorm.io/gorm/clause"

// lockingForUpdate is the row-lock clause used inside transactions to
// serialize operations that must not run concurrently on the same row.
var lockingForUpdate = clause.Locking{Strength: "UPDATE"}
