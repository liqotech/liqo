package interfaces

import (
	"context"
	"sync"
)

// UpdaterInterface represents a generic subset of Updater exported methods to be used instead of a direct access to
// a particular Updater instance.
type UpdaterInterface interface {
	// Start runs an instance of an updater which will be stopped when ctx.Done() is closed.
	Start(ctx context.Context, group *sync.WaitGroup)
	// Push adds the clusterID to the internal queue to be processed as soon as possible.
	Push(clusterID string)
}
