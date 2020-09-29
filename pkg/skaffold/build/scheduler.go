/*
Copyright 2020 The Skaffold Authors

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

package build

import (
	"context"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

// scheduler handles build ordering and concurrency.
type scheduler struct {
	dagMap map[string]artifactDAG
	// sem is a buffered channel that will allow up to `c` concurrent operations when its buffer size is `c`.
	sem chan bool
}

func newScheduler(artifacts []*latest.Artifact, concurrency int) scheduler {
	if concurrency == 0 {
		concurrency = len(artifacts)
	}
	return scheduler{
		dagMap: makeArtifactDAG(artifacts),
		sem:    make(chan bool, concurrency),
	}
}

func (c *scheduler) run(ctx context.Context, a *latest.Artifact, build func() (string, error), onSuccess func(tag string), onError func(err error)) {
	dag := c.dagMap[a.ImageName]
	err := dag.waitForDependencies(ctx)
	c.sem <- true
	if err != nil {
		onError(err)
		dag.markFailure()
		<-c.sem
		return
	}
	tag, err := build()
	if err != nil {
		onError(err)
		dag.markFailure()
		<-c.sem
		return
	}

	onSuccess(tag)
	dag.markSuccess()
	<-c.sem
}
