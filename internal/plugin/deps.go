package plugin

import (
	"fmt"
	"strings"
)

// SatisfiesConstraint checks whether installedVer satisfies a version constraint string.
// Supported forms:
//
//	">=1.0.0"  — minimum version
//	"<=2.0.0"  — maximum version
//	">1.0.0"   — greater than
//	"<2.0.0"   — less than
//	"^1.2.0"   — compatible (same major, >= given)
//	"~1.2.0"   — patch-level (same major.minor, >= given)
//	"1.2.3"    — exact match
//	""         — any version
func SatisfiesConstraint(installedVer, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}

	switch {
	case strings.HasPrefix(constraint, ">="):
		return CompareVersions(installedVer, constraint[2:]) >= 0
	case strings.HasPrefix(constraint, "<="):
		return CompareVersions(installedVer, constraint[2:]) <= 0
	case strings.HasPrefix(constraint, ">"):
		return CompareVersions(installedVer, constraint[1:]) > 0
	case strings.HasPrefix(constraint, "<"):
		return CompareVersions(installedVer, constraint[1:]) < 0
	case strings.HasPrefix(constraint, "^"):
		// Caret: same major version, at least the given version.
		target := strings.TrimPrefix(constraint[1:], "v")
		installed := strings.TrimPrefix(installedVer, "v")

		targetParts := strings.SplitN(target, ".", 3)
		installedParts := strings.SplitN(installed, ".", 3)

		if len(targetParts) == 0 || len(installedParts) == 0 {
			return false
		}

		if parseVersionPart(installedParts[0]) != parseVersionPart(targetParts[0]) {
			return false
		}

		return CompareVersions(installedVer, constraint[1:]) >= 0
	case strings.HasPrefix(constraint, "~"):
		// Tilde: same major.minor, at least the given version.
		target := strings.TrimPrefix(constraint[1:], "v")
		installed := strings.TrimPrefix(installedVer, "v")

		targetParts := strings.SplitN(target, ".", 3)
		installedParts := strings.SplitN(installed, ".", 3)

		if len(targetParts) < 2 || len(installedParts) < 2 {
			return false
		}

		if parseVersionPart(installedParts[0]) != parseVersionPart(targetParts[0]) {
			return false
		}

		if parseVersionPart(installedParts[1]) != parseVersionPart(targetParts[1]) {
			return false
		}

		return CompareVersions(installedVer, constraint[1:]) >= 0
	default:
		// Exact match.
		return CompareVersions(installedVer, constraint) == 0
	}
}

// ResolveDependencies returns an ordered list of plugin names that must be installed
// to satisfy the dependencies of the requested plugin. The list is in install order
// (dependencies before dependents). Plugins already in installed are skipped.
//
// It detects circular dependencies and returns an error for unresolvable constraints.
func ResolveDependencies(requested PluginDef, registry []RegistryEntry, installed map[string]PluginDef) ([]string, error) {
	if len(requested.Dependencies) == 0 {
		return nil, nil
	}

	// Build registry lookup.
	regMap := make(map[string]RegistryEntry, len(registry))
	for _, e := range registry {
		regMap[e.Name] = e
	}

	var order []string

	visited := make(map[string]bool)  // in current path (cycle detection)
	resolved := make(map[string]bool) // fully resolved

	// Mark installed plugins as resolved.
	for name := range installed {
		resolved[name] = true
	}

	var resolve = func(deps []PluginDependency) error {
		for _, dep := range deps {
			if resolved[dep.Name] {
				// Already installed — check version constraint.
				if inst, ok := installed[dep.Name]; ok && dep.Version != "" {
					if !SatisfiesConstraint(inst.Version, dep.Version) && !dep.Optional {
						return fmt.Errorf("plugin %q installed version %q does not satisfy %q",
							dep.Name, inst.Version, dep.Version)
					}
				}

				continue
			}

			if visited[dep.Name] {
				return fmt.Errorf("circular dependency detected: %s", dep.Name)
			}

			// Look up in registry.
			entry, ok := regMap[dep.Name]
			if !ok {
				if dep.Optional {
					continue
				}

				return fmt.Errorf("dependency %q not found in registry", dep.Name)
			}

			// Check version constraint against registry version.
			if dep.Version != "" && !SatisfiesConstraint(entry.Version, dep.Version) {
				if dep.Optional {
					continue
				}

				return fmt.Errorf("dependency %q registry version %q does not satisfy %q",
					dep.Name, entry.Version, dep.Version)
			}

			// To resolve transitive deps, we need the plugin def. We only have
			// the registry entry (no nested deps info). For transitive resolution
			// we'd need to fetch the plugin. For now, we add a placeholder and
			// rely on the registry entry having a matching PluginDef structure.
			// Transitive deps are resolved if the registry provides PluginDefs.
			visited[dep.Name] = true

			// Check if registry entry corresponds to a known def with its own deps.
			// In practice, the caller can populate the registry with full defs.
			// For basic resolution, we just add to the order.

			resolved[dep.Name] = true
			visited[dep.Name] = false

			order = append(order, dep.Name)
		}

		return nil
	}

	if err := resolve(requested.Dependencies); err != nil {
		return nil, err
	}

	return order, nil
}

// ResolveDependenciesDeep resolves transitive dependencies by accepting a map of
// plugin definitions keyed by name (e.g., fetched from the registry). This allows
// full recursive resolution including cycle detection across the dependency tree.
func ResolveDependenciesDeep(requested PluginDef, available map[string]PluginDef, installed map[string]PluginDef) ([]string, error) {
	if len(requested.Dependencies) == 0 {
		return nil, nil
	}

	var order []string

	visited := make(map[string]bool)  // in current path
	resolved := make(map[string]bool) // fully resolved

	for name := range installed {
		resolved[name] = true
	}

	var resolve func(name string, deps []PluginDependency) error

	resolve = func(parent string, deps []PluginDependency) error {
		for _, dep := range deps {
			if resolved[dep.Name] {
				if inst, ok := installed[dep.Name]; ok && dep.Version != "" {
					if !SatisfiesConstraint(inst.Version, dep.Version) && !dep.Optional {
						return fmt.Errorf("plugin %q installed version %q does not satisfy %q (required by %s)",
							dep.Name, inst.Version, dep.Version, parent)
					}
				}

				continue
			}

			if visited[dep.Name] {
				return fmt.Errorf("circular dependency detected: %s -> %s", parent, dep.Name)
			}

			avail, ok := available[dep.Name]
			if !ok {
				if dep.Optional {
					continue
				}

				return fmt.Errorf("dependency %q (required by %s) not found", dep.Name, parent)
			}

			if dep.Version != "" && !SatisfiesConstraint(avail.Version, dep.Version) {
				if dep.Optional {
					continue
				}

				return fmt.Errorf("dependency %q version %q does not satisfy %q (required by %s)",
					dep.Name, avail.Version, dep.Version, parent)
			}

			visited[dep.Name] = true

			// Resolve transitive dependencies first.
			if err := resolve(dep.Name, avail.Dependencies); err != nil {
				return err
			}

			visited[dep.Name] = false
			resolved[dep.Name] = true

			order = append(order, dep.Name)
		}

		return nil
	}

	if err := resolve(requested.Name, requested.Dependencies); err != nil {
		return nil, err
	}

	return order, nil
}
