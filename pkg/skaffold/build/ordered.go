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
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/tag"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/color"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/event"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

const bufferedLinesPerArtifact = 10000

type ArtifactBuilder func(ctx context.Context, out io.Writer, artifact *latest.Artifact, tag string, artifactResolver ArtifactResolver) (string, error)

// For testing
var (
	buffSize = bufferedLinesPerArtifact
)

// InOrder builds a list of artifacts in dependency order.
// `concurrency` specifies the max number of builds that can run at any one time. If concurrency is 0, then it's set to the length of the `artifacts` slice.
// Each artifact build runs in its own goroutine. It limits the max concurrency using a buffered channel like a semaphore.
// At the same time, it uses the `artifactDAG` to model the artifacts dependency graph and to make each artifact build wait for its required artifacts' builds.
func InOrder(ctx context.Context, out io.Writer, tags tag.ImageTags, artifacts []*latest.Artifact, buildArtifact ArtifactBuilder, concurrency int) ([]Artifact, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := new(sync.Map)
	outputs := make([]chan string, len(artifacts))

	scheduler := newScheduler(artifacts, concurrency)
	var wg sync.WaitGroup
	defer wg.Wait()

	wg.Add(len(artifacts))
	for i := range artifacts {
		outputs[i] = make(chan string, buffSize)
		r, w := io.Pipe()

		// Create a goroutine for each element in acmSlice. Each goroutine waits on its dependencies to finish building.
		// Because our artifacts form a DAG, at least one of the goroutines should be able to start building.
		go func(a *latest.Artifact) {
			// Run build and write output/logs to piped writer and store build result in sync.Map
			scheduler.run(ctx, a,
				func() (string, error) {
					event.BuildInProgress(a.ImageName)
					return getBuildResult(ctx, w, tags, a, buildArtifact)
				},
				func(tag string) {
					event.BuildComplete(a.ImageName)
					ar := Artifact{ImageName: a.ImageName, Tag: tag}
					results.Store(ar.ImageName, ar)
					w.Close()
				},
				func(err error) {
					event.BuildFailed(a.ImageName, err)
					results.Store(a.ImageName, err)
					w.Close()
				})
			wg.Done()
		}(artifacts[i])

		// Read build output/logs and write to buffered channel
		go readOutputAndWriteToChannel(r, outputs[i])
	}

	// Print logs and collect results in order.
	return collectResults(out, artifacts, results, outputs)
}

func readOutputAndWriteToChannel(r io.Reader, lines chan string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	close(lines)
}

func getBuildResult(ctx context.Context, cw io.Writer, tags tag.ImageTags, artifact *latest.Artifact, build ArtifactBuilder) (string, error) {
	color.Default.Fprintf(cw, "Building [%s]...\n", artifact.ImageName)
	tag, present := tags[artifact.ImageName]
	if !present {
		return "", fmt.Errorf("unable to find tag for image %s", artifact.ImageName)
	}
	return build(ctx, cw, artifact, tag)
}

func collectResults(out io.Writer, artifacts []*latest.Artifact, results *sync.Map, outputs []chan string) ([]Artifact, error) {
	var built []Artifact
	for i, artifact := range artifacts {
		// Wait for build to complete.
		printResult(out, outputs[i])
		v, ok := results.Load(artifact.ImageName)
		if !ok {
			return nil, fmt.Errorf("could not find build result for image %s", artifact.ImageName)
		}
		switch t := v.(type) {
		case error:
			return nil, fmt.Errorf("couldn't build %q: %w", artifact.ImageName, t)
		case Artifact:
			built = append(built, t)
		default:
			return nil, fmt.Errorf("unknown type %T for %s", t, artifact.ImageName)
		}
	}
	return built, nil
}

func printResult(out io.Writer, output chan string) {
	for line := range output {
		fmt.Fprintln(out, line)
	}
}
