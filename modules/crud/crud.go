package crud

import (
	"context"
	"log"
	"sync"

	"github.com/spaceuptech/space-cloud/config"
	"github.com/spaceuptech/space-cloud/model"
	"github.com/spaceuptech/space-cloud/utils"

	"github.com/spaceuptech/space-cloud/modules/crud/mgo"
	"github.com/spaceuptech/space-cloud/modules/crud/sql"
)

// Module is the root block providing convenient wrappers
type Module struct {
	sync.RWMutex
	blocks             map[string]Crud
	primaryDB          string
	project            string
	removeProjectScope bool

	// Variables to store the hooks
	hooks      *model.CrudHooks
	metricHook model.MetricCrudHook
}

// Crud abstracts the implementation crud operations of databases
type Crud interface {
	Create(ctx context.Context, project, col string, req *model.CreateRequest) (int64, error)
	Read(ctx context.Context, project, col string, req *model.ReadRequest) (int64, interface{}, error)
	Update(ctx context.Context, project, col string, req *model.UpdateRequest) (int64, error)
	Delete(ctx context.Context, project, col string, req *model.DeleteRequest) (int64, error)
	Aggregate(ctx context.Context, project, col string, req *model.AggregateRequest) (interface{}, error)
	Batch(ctx context.Context, project string, req *model.BatchRequest) ([]int64, error)
	DescribeTable(ctc context.Context, project, dbType, col string) ([]utils.FieldType, []utils.ForeignKeysType, error)
	RawExec(ctx context.Context, project string) error
	GetCollections(ctx context.Context, project string) ([]utils.DatabaseCollections, error)
	DeleteCollection(ctx context.Context, project, col string) error
	RawBatch(ctx context.Context, batchedQueries []string) error
	GetDBType() utils.DBType
	IsClientSafe() error
	Close() error
	GetConnectionState(ctx context.Context, dbType string) bool
}

// Init create a new instance of the Module object
func Init(removeProjectScope bool) *Module {
	return &Module{blocks: make(map[string]Crud), removeProjectScope: removeProjectScope}
}

// SetHooks sets the internal hooks
func (m *Module) SetHooks(hooks *model.CrudHooks, metricHook model.MetricCrudHook) {
	m.hooks = hooks
	m.metricHook = metricHook
}

func (m *Module) initBlock(dbType utils.DBType, enabled bool, connection string) (Crud, error) {
	switch dbType {
	case utils.Mongo:
		return mgo.Init(enabled, connection)

	case utils.MySQL, utils.Postgres, utils.SqlServer:
		return sql.Init(dbType, enabled, m.removeProjectScope, connection)
	default:
		return nil, utils.ErrInvalidParams
	}
}

func (m *Module) getCrudBlock(dbType string) (Crud, error) {
	if crud, p := m.blocks[dbType]; p {
		return crud, nil
	}

	return nil, utils.ErrDatabaseConfigAbsent
}

// SetConfig set the rules adn secret key required by the crud block
func (m *Module) SetConfig(project string, crud config.Crud) error {
	m.Lock()
	defer m.Unlock()

	m.project = project

	// Close the previous database connections
	for _, v := range m.blocks {
		v.Close()
	}
	m.blocks = make(map[string]Crud, len(crud))

	// Create a new crud blocks
	for k, v := range crud {
		c, err := m.initBlock(utils.DBType(k), v.Enabled, v.Conn)
		m.blocks[k] = c

		if err != nil {
			log.Println("Error connecting to " + k + " : " + err.Error())
			return err
		} else {
			log.Println("Successfully connected to " + k)
		}
	}
	return nil
}
