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

package docker

import (
	"context"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/docker"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

// Builder is an artifact builder that uses docker
type Builder struct {
	localDocker        docker.LocalDaemon
	pushImages         bool
	prune              bool
	useCLI             bool
	useBuildKit        bool
	mode               config.RunMode
	insecureRegistries map[string]bool
	artifacts          ArtifactResolver
	deps               DependencyResolver
}

// ArtifactResolver provides an interface to resolve built artifact tags by image name.
type ArtifactResolver interface {
	GetImageTag(imageName string) (string, bool)
}

// ArtifactResolver provides an interface to resolve built artifact tags by image name.
type DependencyResolver interface {
	DependenciesForArtifact(ctx context.Context, a *latest.Artifact) ([]string, error)
}

// NewBuilder returns an new instance of a docker builder
func NewArtifactBuilder(localDocker docker.LocalDaemon, useCLI, useBuildKit, pushImages, prune bool, mode config.RunMode, insecureRegistries map[string]bool, ar ArtifactResolver, dr DependencyResolver) *Builder {
	return &Builder{
		localDocker:        localDocker,
		pushImages:         pushImages,
		prune:              prune,
		useCLI:             useCLI,
		useBuildKit:        useBuildKit,
		mode:               mode,
		insecureRegistries: insecureRegistries,
		artifacts:          ar,
		deps:               dr,
	}
}
