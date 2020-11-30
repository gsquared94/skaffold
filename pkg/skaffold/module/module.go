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
	"path/filepath"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
)

// setAbsModuleRefs updates all module refs to be absolute paths.
func setAbsModuleRefs(cfg *latest.SkaffoldConfig, basePath string) {
	// referenced modules
	for _, m := range cfg.Modules {
		m.Path = absModulePath(m.Path, basePath)
	}

	// module profiles
	for _, p := range cfg.Profiles {
		for _, mp := range p.ModuleProfiles {
			mp.Path = absModulePath(mp.Path, basePath)
		}
	}

	// implicit module dependencies
	for _, a := range cfg.Build.Artifacts {
		for _, d := range a.Dependencies {
			d.Module.Path = absModulePath(d.Module.Path, basePath)
		}
	}

	// inline module dependencies
	for _, m := range cfg.Modules {
		for _, a := range m.Build.Artifacts {
			for _, d := range a.Dependencies {
				d.Module.Path = absModulePath(d.Module.Path, basePath)
			}
		}
	}

	// profile override dependencies
	for _, p := range cfg.Profiles {
		for _, a := range p.Build.Artifacts {
			for _, d := range a.Dependencies {
				d.Module.Path = absModulePath(d.Module.Path, basePath)
			}
		}
	}
}

func absModulePath(path string, basePath string) string {
	if path == "" {
		return basePath
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(basePath), path)
	}
	ext := filepath.Ext(path)
	if ext == "" {
		path = filepath.Join(path, "skaffold.yaml")
	}
	return path
}
