// services/api-gateway/internal/graphql/resolver.go
package graphql

import (
	"github.com/kisna/archon/services/api-gateway/internal/kafka"
	"github.com/kisna/archon/services/api-gateway/internal/db"
	"github.com/kisna/archon/services/api-gateway/internal/grpcclient"
	"github.com/kisna/archon/services/api-gateway/internal/ws"
)

// This file will not be regenerated automatically.
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	DB *db.Repository
	AI *grpcclient.ArchitectClient
	Kafka  *kafka.Producer
	Redis *ws.RedisManager
}