package api

import "context"

type DdaAssignment struct {
	Exists           bool
	VMName           string
	ResourcePoolName string
	LocationPath     string
	InstancePath     string
}

type HypervDdaAssignmentClient interface {
	GetDdaAssignment(ctx context.Context, vmName, resourcePoolName string) (DdaAssignment, error)
	CreateDdaAssignment(ctx context.Context, vmName, resourcePoolName string) (DdaAssignment, error)
	DeleteDdaAssignment(ctx context.Context, vmName, resourcePoolName, locationPath string) error
}
