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

package build

import (
	"context"
	"fmt"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/bazel"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/buildpacks"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/custom"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/jib"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/build/misc"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

// DependenciesForArtifact returns the dependencies for a given artifact.
func DependenciesForArtifact(ctx context.Context, a *latest.Artifact, cfg docker.Config, dependencies []*latest.ArtifactDependency, r ArtifactResolver) ([]string, error) {
	var (
		paths []string
		err   error
	)

	switch {
	case a.DockerArtifact != nil:
		deps, err := docker.ResolveArtifactDependencies(dependencies, r)
		if err != nil {
			return nil, fmt.Errorf("unable to resolve required artifacts: %w", err)
		}
		buildArgs, err := docker.EvalBuildArgs(cfg.Mode(), a.Workspace, a.DockerArtifact, deps)
		if err != nil {
			return nil, fmt.Errorf("unable to evaluate build args: %w", err)
		}
		paths, err = docker.GetDependencies(ctx, a.Workspace, a.DockerArtifact.DockerfilePath, buildArgs, cfg)

	case a.KanikoArtifact != nil:
		paths, err = docker.GetDependencies(ctx, a.Workspace, a.KanikoArtifact.DockerfilePath, a.KanikoArtifact.BuildArgs, cfg)

	case a.BazelArtifact != nil:
		paths, err = bazel.GetDependencies(ctx, a.Workspace, a.BazelArtifact)

	case a.JibArtifact != nil:
		paths, err = jib.GetDependencies(ctx, a.Workspace, a.JibArtifact)

	case a.CustomArtifact != nil:
		paths, err = custom.GetDependencies(ctx, a.Workspace, a.CustomArtifact, cfg)

	case a.BuildpackArtifact != nil:
		paths, err = buildpacks.GetDependencies(ctx, a.Workspace, a.BuildpackArtifact)

	default:
		return nil, fmt.Errorf("unexpected artifact type %q:\n%s", misc.ArtifactType(a), misc.FormatArtifact(a))
	}

	if err != nil {
		return nil, err
	}

	return util.AbsolutePaths(a.Workspace, paths), nil
}

func getBuildArgs(a *latest.DockerArtifact, workspace string, mode config.RunMode, dependencies []*latest.ArtifactDependency, r ArtifactResolver) (map[string]*string, error) {

}
