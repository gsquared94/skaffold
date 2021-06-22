/*
Copyright 2019 The Skaffold Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gcb

import (
	"context"
	"io"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/graph"
	latestV1 "github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest/v1"
)

const (
	// StatusUnknown "STATUS_UNKNOWN" - Status of the build is unknown.
	StatusUnknown = "STATUS_UNKNOWN"

	// StatusQueued "QUEUED" - Build is queued; work has not yet begun.
	StatusQueued = "QUEUED"

	// StatusWorking "WORKING" - Build is being executed.
	StatusWorking = "WORKING"

	// StatusSuccess  "SUCCESS" - Build finished successfully.
	StatusSuccess = "SUCCESS"

	// StatusFailure  "FAILURE" - Build failed to complete successfully.
	StatusFailure = "FAILURE"

	// StatusInternalError  "INTERNAL_ERROR" - Build failed due to an internal cause.
	StatusInternalError = "INTERNAL_ERROR"

	// StatusTimeout  "TIMEOUT" - Build took longer than was allowed.
	StatusTimeout = "TIMEOUT"

	// StatusCancelled  "CANCELLED" - Build was canceled by a user.
	StatusCancelled = "CANCELLED"

	// PollDelay is the time to wait in between writing logs of the cloud build
	PollDelay = 1 * time.Second

	// BackoffFactor is the exponent for exponential backoff during build status polling
	BackoffFactor = 1.5

	// BackoffSteps is the number of times we increase the backoff time during exponential backoff
	BackoffSteps = 10

	// MaxRetryCount is the max number of times we retry a throttled GCB API request
	MaxRetryCount = 10
)

func NewStatusBackoff() *wait.Backoff {
	return &wait.Backoff{
		Duration: PollDelay,
		Factor:   float64(BackoffFactor),
		Steps:    BackoffSteps,
		Cap:      10 * time.Second,
	}
}

// Builder builds artifacts with Google Cloud Build.
type Builder struct {
	*latestV1.GoogleCloudBuild

	cfg                Config
	skipTests          bool
	muted              build.Muted
	artifactStore      build.ArtifactStore
	sourceDependencies graph.SourceDependenciesCache
	reporter           statusManager
}

type Config interface {
	docker.Config

	SkipTests() bool
	Muted() config.Muted
}

type BuilderContext interface {
	Config
	ArtifactStore() build.ArtifactStore
	SourceDependenciesResolver() graph.SourceDependenciesCache
}

// NewBuilder creates a new Builder that builds artifacts with Google Cloud Build.
func NewBuilder(bCtx BuilderContext, buildCfg *latestV1.GoogleCloudBuild) *Builder {
	return &Builder{
		GoogleCloudBuild:   buildCfg,
		cfg:                bCtx,
		skipTests:          bCtx.SkipTests(),
		muted:              bCtx.Muted(),
		artifactStore:      bCtx.ArtifactStore(),
		sourceDependencies: bCtx.SourceDependenciesResolver(),
		reporter:           getStatusManager(),
	}
}

func (b *Builder) Prune(ctx context.Context, out io.Writer) error {
	return nil // noop
}
