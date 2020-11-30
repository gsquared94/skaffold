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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/config"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/latest"
	sutil "github.com/GoogleContainerTools/skaffold/pkg/skaffold/schema/util"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/update"
	"github.com/GoogleContainerTools/skaffold/pkg/skaffold/util"
	"github.com/sirupsen/logrus"
)

func Parse(configFile string, opts config.SkaffoldOptions) (*latest.SkaffoldConfig, error) {
	modPath := configFile
	if configFile == "-" {
		// if config is read from `os.Stdin` then set the current working directory as the base path to resolve all relative paths for modules.
		modPath, _ = os.Getwd()
	}
	cfg, err := parse(latest.ModuleRef{Path: modPath}, configFile, make(map[latest.ModuleRef]*Config), inferredSelector{opts: opts}, util.NewSyncStore())
	if err != nil {

		// If the error is NOT that the file doesn't exist, then we warn the user
		// that maybe they are using an outdated version of Skaffold that's unable to read
		// the configuration.
		warnIfUpdateIsAvailable(opts)
		return nil, fmt.Errorf("parsing skaffold config: %w", err)
	}
	return cfg, err
}

func parse(target latest.ModuleRef, configFile string, store map[latest.ModuleRef]*Config, p profileSelector, once *util.SyncStore) (*latest.SkaffoldConfig, error) {
	parsed, err := readConfig(configFile, once)
	if err != nil {
		if os.IsNotExist(errors.Unwrap(err)) {
			return nil, fmt.Errorf("skaffold config file %s not found - check your current working directory, or module definition", configFile)
		}
		return nil, fmt.Errorf("parsing skaffold config: %w", err)
	}

	cfg := parsed.(*latest.SkaffoldConfig)
	profiles, err := p.ActiveProfiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("activating profiles for skaffold config file %s: %w", configFile, err)
	}

	if c, found := store[target]; found {
		return c.cfg, checkProfiles(c.profiles, profiles)
	}

	setAbsModuleRefs(cfg, target.Path)

	if err = schema.ResolveProfiles(cfg, profiles); err != nil {
		return nil, fmt.Errorf("applying profiles for skaffold config file %s: %w", configFile, err)
	}

	inlineModules := make(map[latest.ModuleRef]latest.Module)
	for _, m := range cfg.Modules {
		if m.Path == target.Path {
			inlineModules[m.ModuleRef] = m
		}
	}

	var parentCfg *latest.SkaffoldConfig
	var moduleCfgs []*latest.SkaffoldConfig
	var nextTargets []latest.Module

	if target.Name == "" {
		// empty module reference name indicates that the entire `skaffold.yaml` is required.
		store[target] = &Config{
			cfg:      cfg,
			profiles: profiles,
		}
		parentCfg = cfg
		nextTargets = append(nextTargets, cfg.Modules...)
		mainPipelineDeps, err := dependencyModules(cfg.Build.Artifacts, inlineModules, target)
		if err != nil {
			return nil, err
		}
		nextTargets = append(nextTargets, mainPipelineDeps...)
		for _, m := range inlineModules {
			modulePipelineDeps, err := dependencyModules(m.Build.Artifacts, inlineModules, target)
			if err != nil {
				return nil, err
			}
			nextTargets = append(nextTargets, modulePipelineDeps...)
		}
	} else {
		// named target module must exist in the inline modules
		m, found := inlineModules[target]
		if !found {
			return nil, fmt.Errorf("cannot find module %s in skaffold config file %s", target.Name, configFile)
		}
		nextTargets = append(nextTargets, m)
		modulePipelineDeps, err := dependencyModules(m.Build.Artifacts, inlineModules, target)
		if err != nil {
			return nil, err
		}
		nextTargets = append(nextTargets, modulePipelineDeps...)
	}

	moduleProfiles := getModuleProfiles(cfg.Profiles, profiles)

	// recurse for each module reference
	for _, m := range nextTargets {
		var moduleCfg *latest.SkaffoldConfig
		if m.ModuleRef.Path == "" {
			// inline modules can be wrapped in a `SkaffoldConfig` and added directly
			m.ModuleRef.Path = target.Path
			moduleCfg = &latest.SkaffoldConfig{
				APIVersion: cfg.APIVersion,
				Kind:       cfg.Kind,
				Metadata:   cfg.Metadata,
				Pipeline:   m.Pipeline,
			}
			store[m.ModuleRef] = &Config{
				cfg: moduleCfg,
			}
		} else {
			moduleCfg, err = parse(m.ModuleRef, m.Path, store, listSelector(moduleProfiles[m.ModuleRef]), once)
			if err != nil {
				return nil, fmt.Errorf("failed to process module %s for skaffold config %s: %w", m.Name, m.Path, err)
			}
		}
		moduleCfgs = append(moduleCfgs, moduleCfg)
	}
	return merge(parentCfg, moduleCfgs)
}

type Config struct {
	cfg      *latest.SkaffoldConfig
	profiles []string
}

func warnIfUpdateIsAvailable(opts config.SkaffoldOptions) {
	warning, err := update.CheckVersionOnError(opts.GlobalConfig)
	if err != nil {
		logrus.Infof("update check failed: %s", err)
		return
	}
	if warning != "" {
		logrus.Warn(warning)
	}
}

// getDir returns the directory for the `skaffold.yaml` if it's a file. If it's read from `os.Stdin` then it simply return `-`
func getDir(filename string) string {
	switch filename {
	case "-":
		return filepath.Dir(os.Args[0])
	default:
		return filepath.Dir(filename)
	}
}

// dependencyModules returns all modules in the transitive closure on the source module in the graph of module dependencies.
func dependencyModules(artifacts []*latest.Artifact, inlineModules map[latest.ModuleRef]latest.Module, target latest.ModuleRef) ([]latest.Module, error) {
	m := make(map[latest.ModuleRef]latest.Module)
	// empty module reference name indicates that the entire `skaffold.yaml` is required.
	for _, a := range artifacts {
		for _, d := range a.Dependencies {
			if d.Module.Path == target.Path {
				// required module is defined inline
				mo, found := inlineModules[d.Module]
				if !found {
					return nil, fmt.Errorf("cannot find module %s in skaffold config file %s", mo.Name, target.Path)
				}

				// handle circular references
				if _, added := m[mo.ModuleRef]; added {
					continue
				}
				m[mo.ModuleRef] = mo
				// recursively add inline module dependencies
				deps, err := dependencyModules(mo.Build.Artifacts, inlineModules, target)
				if err != nil {
					return nil, err
				}
				for _, d := range deps {
					m[d.ModuleRef] = d
				}
			} else {
				// required module is not inline
				m[d.Module] = latest.Module{ModuleRef: d.Module}
			}
		}
	}
	var sl []latest.Module
	for _, v := range m {
		sl = append(sl, v)
	}
	return sl, nil
}

func readConfig(file string, once *util.SyncStore) (sutil.VersionedConfig, error) {
	val := once.Exec(file, func() interface{} {
		c, err := schema.ParseConfigAndUpgrade(file, latest.Version)
		if err != nil {
			return err
		}
		return c
	})

	if err, ok := val.(error); ok {
		return nil, err
	}
	return val.(sutil.VersionedConfig), nil
}
