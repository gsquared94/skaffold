# Implementing Skaffold Modules

* Author: Gaurav Ghosh
* Design Shepherd: Brian de Alwis
* Date: 2020-11-09
* Status: Reviewed

## Background

Refer `design_proposals/modules.md` which presented the design goals for introducing the concept of _modules_ in Skaffold. 

The current document aims to capture the major code changes necessary for realizing the aspirations of `design_proposals/modules.md`.

In `design_proposals/artifact-dependencies.md` we implemented the feature of inter-artifact dependencies- build ordering, cache handling & invalidation, dev-loop integration, build log ordering, among other things.
That completed the most important pre-requisite features to be able to now implement `module` as a skaffold schema construct.

>*Note: Omitted some details for brevity, assuming reader's familiarity with the skaffold [codebase](https://github.com/GoogleContainerTools/skaffold).*

___

## Design

### Config Schema

We modify `latest.SkaffoldConfig`, as:

```go
type SkaffoldConfig struct {
	// existing properties.

	// Modules *alpha* describe references to other skaffold pipelines.
	Modules []Module `yaml:"modules,omitempty"`
}
```
`latest.ArtifactDependency`, as:

```go
type ArtifactDependency struct {
	// existing properties `imageName` and `alias`

	// Module identifies the module that the specified required artifact belongs to.
	Module ModuleRef `yaml:"module,omitempty"`
}
```
and `latest.Profile`, as:

```go
type Profile struct {
	// existing properties `name`, `activation`, `patches` and `pipeline`.

	// ModuleProfiles defines profiles to activate in referenced modules when this profile gets activated
	ModuleProfiles []ModuleProfile `yaml:"modules,omitempty"`
}
```
where `latest.Module`, `latest.ModuleProfile` and `latest.ModuleRef` are defined, as:

```go
// Module describes a module definition
type Module struct {
	// ModuleRef identifies this module definition
	ModuleRef `yaml:",inline"`
	// Pipeline defines the Build/Test/Deploy phases.
	Pipeline `yaml:",inline"`
}

// ModuleProfile defines a subset of profiles in the specified module to enable.
type ModuleProfile struct {
	// ModuleRef identifies current module definition.
	ModuleRef `yaml:",inline"`

	// Active identifies profiles in current module that should be active.
	Active []string `yaml:"activeProfiles,omitempty"`
}

// ModuleRef identifies a module either by name or path. 
type ModuleRef struct {
	// Name is a unique module name.
	Name string `yaml:"name,omitempty"`

	// Path is a relative path to the skaffold.yaml describing this module.
	Path string `yaml:"path,omitempty"`
}
```

This allows us to define `modules` in both the `inline` and `distributed` modes in a  `skaffold.yaml`, as:

_Distributed_:
```yaml
apiVersion: skaffold/v2beta10
kind: Config
build:
 artifacts:
  - image: img
modules:
# referring to modules defined in other directory
 - path: some/relative/path
   name: module1
 - path: some/relative/path
   name: module2
profiles:
 - name: gcb
    build:
      googleCloudBuild:
        projectId: k8s-skaffold
   modules:
     - path: some/relative/path
       name: module1
       activeProfiles:
         - gcb
     - path: some/relative/path
       name: module2
       activeProfiles:
         - gcb
```

_Inline_:
```yaml
apiVersion: skaffold/v2beta10
kind: Config
modules:
# referring to modules defined inline
 - name: module1
   build:
     artifacts:
       - image: img1
         context: ./path1/
         requires:
           - alias: IMG2
             image: img2
             module:
               name: module2
   deploy:
     kubectl:
       manifests:
         - ./path1/kubernetes/*
   portForward:
     - resourceType: deployment
       resourceName: img1
       port: 8080
       localPort: 9000
 - name: module2
   build:
     artifacts:
       - image: img2
         context: ./path2/
   deploy:
     kubectl:
       manifests:
         - ./path2/kubernetes/*
profiles:
 - name: gcb
   build:
    googleCloudBuild:
      projectId: k8s-skaffold
```

### Stitching together multiple `skaffold.yaml` files

In `pkg/skaffold/modules/parse.go` we define:

```go
// parse reads the configFile as the target skaffold config and merges all referenced modules into it as single `latest.SkaffoldConfig`
func parse(target latest.ModuleRef, configFile string, store map[latest.ModuleRef]*Config, p profileSelector, once *util.SyncStore) (*latest.SkaffoldConfig, error)
```

where `Config` wraps any intermediate skaffold config along with the profiles that were activated on it:

```go
type Config struct {
	cfg      *latest.SkaffoldConfig
	profiles []string
} 
```


In `pkg/skaffold/modules/merge.go` we define:

```go
// merge processes a parent module and a slice of referenced modules into a single skaffold module.
func merge(parent *latest.SkaffoldConfig, modules []*latest.SkaffoldConfig) (*latest.SkaffoldConfig, error)
```

#### Parse algorithm

1. Read config and get the active profile list. 
2. Validate that the same config wasn't previously processed for a different active profile list (check `store`). If the profile list is different, then return an error, else return the value from `store`.
3. a. If the current module is named, then search for it in the inline modules. Additionally, search for all modules referenced in this target module along with all modules that occur as artifact dependencies.
3. b. Call `merge` with this list of modules (and empty parent) to resolve a single config and return.
4. a. Else if the current module is unnamed, then it refers to the whole config. So add all modules in this config along with all modules that occur as artifact dependencies.
4. b. Call `merge` with this list of modules, and the current config as the parent module to resolve a single config and return.
5. In 3.a or 4.a for the modules that are references to skaffold configs defined in other files, recursively call this `parse` function to resolve the module config.




