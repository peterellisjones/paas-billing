package cfstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"code.cloudfoundry.org/lager"
	cfclient "github.com/cloudfoundry-community/go-cfclient"
)

const (
	DefaultInitTimeout = 5 * time.Minute
)

type Config struct {
	// CFClient config
	ClientConfig *cfclient.Config
	// Client for communicating with cf
	Client CFDataClient
	// Database connection
	DB *sql.DB
	// Logger overrides the default logger
	Logger lager.Logger
	// Collection delay
	Schedule time.Duration
}

type Store struct {
	client CFDataClient
	db     *sql.DB
	logger lager.Logger
}

func (s *Store) Init() error {
	s.logger.Info("initializing")
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.collectServices(tx); err != nil {
		return err
	}
	if err := s.collectServicePlans(tx); err != nil {
		return err
	}
	if err := s.collectSpaces(tx); err != nil {
		return err
	}
	if err := s.collectOrgs(tx); err != nil {
		return err
	}
	s.logger.Info("initialized")
	return tx.Commit()
}

func (s *Store) CollectServicePlans() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.collectServicePlans(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) collectServicePlans(tx *sql.Tx) error {
	plans, err := s.client.ListServicePlans()
	if err != nil {
		return err
	}
	for _, plan := range plans {
		validFrom := plan.UpdatedAt
		var planCount int
		err := tx.QueryRow(`
			select count(*)
			from service_plans
			where guid = $1
		`, plan.Guid).Scan(&planCount)
		if err != nil {
			return err
		}
		if planCount == 0 {
			validFrom = plan.CreatedAt
		}

		var serviceValidFrom *time.Time
		err = tx.QueryRow(`
			select valid_from
			from services
			where guid = $1
			order by valid_from desc
			limit 1
		`, plan.ServiceGuid).Scan(&serviceValidFrom)
		if err == sql.ErrNoRows {
			s.logger.Error("service-not-found", fmt.Errorf("failed to find service '%s' for service_plan '%s'... skipping", plan.ServiceGuid, plan.Guid))
			continue
		} else if err != nil {
			return err
		}

		_, err = tx.Exec(`
			insert into service_plans (
				guid, valid_from,
				name, description,
				unique_id,
				active, public, free,
				extra,
				created_at, updated_at,
				service_guid, service_valid_from
			) values (
				$1, $2,
				$3, $4,
				$5,
				$6, $7, $8,
				$9,
				$10, $11,
				$12, $13
			) on conflict (guid, valid_from) do nothing
		`, plan.Guid, validFrom,
			plan.Name, plan.Description,
			plan.UniqueId,
			plan.Active, plan.Public, plan.Free,
			plan.Extra,
			plan.CreatedAt, plan.UpdatedAt,
			plan.ServiceGuid, serviceValidFrom)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CollectServices() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.collectServices(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) collectServices(tx *sql.Tx) error {
	services, err := s.client.ListServices()
	if err != nil {
		return err
	}
	for _, service := range services {
		validFrom := service.UpdatedAt
		var serviceCount int
		err := tx.QueryRow(`
			select count(*)
			from services
			where guid = $1
		`, service.Guid).Scan(&serviceCount)
		if err != nil {
			return err
		}
		if serviceCount == 0 {
			validFrom = service.CreatedAt
		}

		_, err = tx.Exec(`
			insert into services (
				guid, valid_from,
				label, description,
				active, bindable,
				service_broker_guid,
				created_at, updated_at
			) values (
				$1, $2,
				$3, $4,
				$5, $6,
				$7,
				$8, $9
			) on conflict (guid, valid_from) do nothing
		`, service.Guid, validFrom,
			service.Label, service.Description,
			service.Active, service.Bindable,
			service.ServiceBrokerGuid,
			service.CreatedAt, service.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CollectSpaces() error {
	//fmt.Println("hello") // DEBUG FIXME
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.collectSpaces(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) collectSpaces(tx *sql.Tx) error {
	spaces, err := s.client.ListSpaces()
	if err != nil {
		return err
	}
	for _, spaces := range spaces {
		validFrom := spaces.UpdatedAt
		var spaceCount int
		err := tx.QueryRow(`
			select count(*)
			from spaces
			where guid = $1
		`, spaces.Guid).Scan(&spaceCount)
		if err != nil {
			return err
		}
		if spaceCount == 0 {
			validFrom = spaces.CreatedAt
		}

		_, err = tx.Exec(`
			insert into spaces (
				guid, valid_from,
				space_name, organization_guid,
				org_url, quota_definition_guid,
				isolation_segment_guid,
				created_at, updated_at
			) values (
				$1, $2,
				$3, $4,
				$5, $6,
				$7,
				$8, $9
			) on conflict (guid, valid_from) do nothing
		`, spaces.Guid, validFrom,
			spaces.Name, spaces.OrganizationGuid,
			spaces.OrgURL, spaces.QuotaDefinitionGuid,
			spaces.IsolationSegmentGuid,
			spaces.CreatedAt, spaces.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CollectOrgs() error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultInitTimeout)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.collectOrgs(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) collectOrgs(tx *sql.Tx) error {
	orgs, err := s.client.ListOrgs()
	if err != nil {
		return err
	}
	for _, orgs := range orgs {
		validFrom := orgs.UpdatedAt
		var orgCount int
		err := tx.QueryRow(`
			select count(*)
			from orgs
			where guid = $1
		`, orgs.Guid).Scan(&orgCount)
		if err != nil {
			return err
		}
		if orgCount == 0 {
			validFrom = orgs.CreatedAt
		}

		_, err = tx.Exec(`
			insert into orgs (
				guid, valid_from,
				org_name, quota_definition_guid,
				default_isolation_segment_guid,
				created_at, updated_at
			) values (
				$1, $2,
				$3, $4,
				$5,
				$6, $7
			) on conflict (guid, valid_from) do nothing
		`, orgs.Guid, validFrom,
			orgs.Name, orgs.QuotaDefinitionGuid,
			orgs.DefaultIsolationSegmentGuid,
			orgs.CreatedAt, orgs.UpdatedAt)
		if err != nil {
			return err
		}
	}
	return nil
}

func New(cfg Config) (*Store, error) {
	if cfg.Logger == nil {
		cfg.Logger = lager.NewLogger("historic-data-store")
	}
	store := &Store{
		client: cfg.Client,
		logger: cfg.Logger,
		db:     cfg.DB,
	}
	return store, nil
}
