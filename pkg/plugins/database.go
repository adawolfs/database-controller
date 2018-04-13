package plugins

import (
	adawolfs "github.com/adawolfs/database-controller/pkg/apis/adawolfs.com/v1"
)


type DatabaseHandler interface {
	AddDatabase(db *adawolfs.Database)
	DeleteDatabase(db *adawolfs.Database)
	UpdateDatabase(db *adawolfs.Database)
}
