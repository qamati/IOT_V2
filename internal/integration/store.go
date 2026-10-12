package integration

import (
	"context"

	"github.com/qamati/IOT_V2/pkg/ids"
)

// Store defines persistent storage operations for Converters, Integrations, and Statuses.
type Store interface {
	CreateConverter(ctx context.Context, c *Converter) error
	GetConverter(ctx context.Context, tenantID, id ids.ID) (*Converter, error)
	ListConverters(ctx context.Context, tenantID ids.ID) ([]*Converter, error)
	UpdateConverter(ctx context.Context, c *Converter) error
	DeleteConverter(ctx context.Context, tenantID, id ids.ID) error

	CreateIntegration(ctx context.Context, i *Integration) error
	GetIntegration(ctx context.Context, tenantID, id ids.ID) (*Integration, error)
	GetIntegrationByRoutingKey(ctx context.Context, routingKey string) (*Integration, error)
	ListIntegrations(ctx context.Context, tenantID ids.ID) ([]*Integration, error)
	UpdateIntegration(ctx context.Context, i *Integration) error
	DeleteIntegration(ctx context.Context, tenantID, id ids.ID) error

	UpsertStatus(ctx context.Context, status *IntegrationStatus) error
	GetStatus(ctx context.Context, integrationID ids.ID) (*IntegrationStatus, error)
}
