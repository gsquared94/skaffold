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

package module

import (
	"fmt"
	"sort"
	"strings"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
)

type profileSelector interface {
	ActiveProfiles(cfg *latest.SkaffoldConfig) ([]string, error)
}

type listSelector []string

func (l listSelector) ActiveProfiles(_ *latest.SkaffoldConfig) ([]string, error) {
	return l, nil
}

type inferredSelector struct {
	opts config.SkaffoldOptions
}

func (i inferredSelector) ActiveProfiles(cfg *latest.SkaffoldConfig) ([]string, error) {
	return schema.ActiveProfiles(cfg, i.opts)
}

// checkProfiles returns an error if the same module is being processed again with a different selection of profiles.
func checkProfiles(prev []string, curr []string) error {
	if len(curr) != len(prev) {
		return fmt.Errorf("profile selection mismatch: %s, %s", strings.Join(prev, ","), strings.Join(curr, ","))
	}
	sort.Strings(prev)
	sort.Strings(curr)

	for i := range prev {
		if prev[i] != curr[i] {
			return fmt.Errorf("profile selection mismatch: %s, %s", strings.Join(prev, ","), strings.Join(curr, ","))
		}
	}
	return nil
}

// getModuleProfiles returns the map of active module profiles keyed on the module reference.
func getModuleProfiles(all []latest.Profile, active []string) map[latest.ModuleRef][]string {
	m := make(map[latest.ModuleRef][]string)
	for _, profile := range all {
		if util.StrSliceContains(active, profile.Name) {
			for _, mp := range profile.ModuleProfiles {
				m[mp.ModuleRef] = append(m[mp.ModuleRef], mp.Active...)
			}
		}
	}
	return m
}
