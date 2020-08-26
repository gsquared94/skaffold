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
	"io"
	"sync"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/tag"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

func InDependencyOrder(ctx context.Context, out io.Writer, tags tag.ImageTags, artifacts []*latest.Artifact, buildArtifact ArtifactBuilder, concurrency int) ([]Artifact, error) {
	m := make(map[string]chan interface{})
	for _, a := range artifacts {
		m[a.ImageName] = make(chan interface{})
	}

	awd := make([]artifactWithDeps, len(artifacts), len(artifacts))
	for _, a:= range artifacts {
		awd := artifactWithDeps{Artifact: a}
		for _, d := range a.Dependencies {
			awd.Deps = append(awd.Deps, m[d.ImageName])
		}
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := new(sync.Map)
	outputs := make([]chan string, len(artifacts))

	if concurrency == 0 {
		concurrency = len(artifacts)
	}
	sem := make(chan bool, concurrency)

	// Run builds in //
	wg.Add(len(artifacts))
}

type artifactWithDeps struct {
	Artifact *latest.Artifact
	Deps []chan interface{}
}
