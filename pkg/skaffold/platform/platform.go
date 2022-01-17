/*
Copyright 2021 The Skaffold Authors

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

package platform

import (
	"github.com/containerd/containerd/platforms"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

var (
	All = Platforms{{all: true}}
)

// Platform describes the platform which the image runs on.
type Platform struct {
	all bool
	specs.Platform
}

// Platforms describes a collection of `Platform` structs
type Platforms []Platform

func Parse(platform string) (Platform, error) {
	p, err := platforms.Parse(platform)
	return Platform{Platform: p}, err
}

func (p Platform) String() string {
	return platforms.Format(p.Platform)
}

// Intersect returns the intersection set of Platforms
func (ps Platforms) Intersect(other Platforms) Platforms {
	if len(ps) == 0 {
		return other
	}
	if len(other) == 0 {
		return ps
	}
	var result Platforms
	for i := 0; i < len(ps); i++ {
		if ps[i].all {
			return other
		}
		matcher := platforms.OnlyStrict(ps[i].Platform)
		for j := i; j < len(other); j++ {
			if other[j].all {
				return ps
			}
			if matcher.Match(other[j].Platform) {
				result = append(result, ps[i])
			}
		}
	}
	return result
}
