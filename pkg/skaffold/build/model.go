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
	"fmt"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

// artifactDAG models the artifact dependency graph using a set of channels.
// Each artifact has a status struct that has success and a failure channel which it closes once it completes building by calling either markSuccess or markFailure respectively.
// This notifies all listeners waiting for this artifact of a successful or failed build.
// Additionally it has a reference to the channels for each of its dependencies.
// Calling `waitForDependencies` ensures that all required artifacts' channels have already been closed and as such have finished building before the current artifact build starts.
type artifactDAG struct {
	status             status
	dependencyStatuses []status
}

type status struct {
	imageName string
	success   chan interface{}
	failure   chan interface{}
}

func (a *artifactDAG) markSuccess() {
	// closing channel notifies all listeners waiting for this build that it succeeded
	close(a.status.success)
}

func (a *artifactDAG) markFailure() {
	// closing channel notifies all listeners waiting for this build that it failed
	close(a.status.failure)
}
func (a *artifactDAG) waitForDependencies(ctx context.Context) error {
	for _, depStatus := range a.dependencyStatuses {
		// wait for required builds to complete
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-depStatus.failure:
			return fmt.Errorf("failed to build required artifact: %q", depStatus.imageName)
		case <-depStatus.success:
		}
	}
	return nil
}

func makeArtifactDAG(artifacts []*latest.Artifact) map[string]artifactDAG {
	statusMap := make(map[string]status)
	for _, a := range artifacts {
		statusMap[a.ImageName] = status{
			imageName: a.ImageName,
			success:   make(chan interface{}),
			failure:   make(chan interface{}),
		}
	}

	m := make(map[string]artifactDAG)
	for _, a := range artifacts {
		dag := artifactDAG{status: statusMap[a.ImageName]}
		for _, d := range a.Dependencies {
			dag.dependencyStatuses = append(dag.dependencyStatuses, statusMap[d.ImageName])
		}
		m[a.ImageName] = dag
	}
	return m
}
