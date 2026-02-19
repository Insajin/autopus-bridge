// Package project handles multi-project configuration management.
// FR-P4-04: Multi-project support for the Local Agent Bridge.
package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// Project represents a configured project with its own connection settings.
type Project struct {
	Name      string `yaml:"name" json:"name"`
	Path      string `yaml:"path" json:"path"`
	Workspace string `yaml:"workspace,omitempty" json:"workspace,omitempty"`
	Provider  string `yaml:"provider,omitempty" json:"provider,omitempty"`
	Active    bool   `yaml:"active" json:"active"`
}

// projectsFile represents the YAML structure for persisting projects.
type projectsFile struct {
	Projects []Project `yaml:"projects"`
}

// Manager handles multiple project configurations.
type Manager struct {
	configDir string
	projects  map[string]*Project
	mu        sync.RWMutex
}

// NewManager creates a new project Manager with the given config directory.
func NewManager(configDir string) *Manager {
	return &Manager{
		configDir: configDir,
		projects:  make(map[string]*Project),
	}
}

// projectsFilePath returns the path to the projects configuration file.
func (m *Manager) projectsFilePath() string {
	return filepath.Join(m.configDir, "projects.yaml")
}

// LoadProjects loads project configurations from the projects.yaml file.
// If the file does not exist, the manager starts with an empty project list.
func (m *Manager) LoadProjects() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	filePath := m.projectsFilePath()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No projects file yet; start with empty map.
			m.projects = make(map[string]*Project)
			return nil
		}
		return fmt.Errorf("failed to read projects file: %w", err)
	}

	var pf projectsFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		return fmt.Errorf("failed to parse projects file: %w", err)
	}

	m.projects = make(map[string]*Project, len(pf.Projects))
	for i := range pf.Projects {
		p := pf.Projects[i]
		m.projects[p.Name] = &p
	}

	return nil
}

// AddProject adds a new project configuration.
// Returns an error if a project with the same name already exists.
func (m *Manager) AddProject(p Project) error {
	if p.Name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if p.Path == "" {
		return fmt.Errorf("project path cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[p.Name]; exists {
		return fmt.Errorf("project %q already exists", p.Name)
	}

	m.projects[p.Name] = &p
	return nil
}

// RemoveProject removes a project by name.
// Returns an error if the project does not exist.
func (m *Manager) RemoveProject(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[name]; !exists {
		return fmt.Errorf("project %q not found", name)
	}

	delete(m.projects, name)
	return nil
}

// GetProject retrieves a project by name.
func (m *Manager) GetProject(name string) (*Project, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.projects[name]
	if !ok {
		return nil, false
	}
	// Return a copy to avoid race conditions.
	copy := *p
	return &copy, true
}

// ListProjects returns all configured projects sorted by name.
func (m *Manager) ListProjects() []Project {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Project, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// SetActive sets the given project as the active project.
// Any previously active project is deactivated.
// Returns an error if the project does not exist.
func (m *Manager) SetActive(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.projects[name]
	if !exists {
		return fmt.Errorf("project %q not found", name)
	}

	// Deactivate all projects first.
	for _, p := range m.projects {
		p.Active = false
	}

	target.Active = true
	return nil
}

// GetActive returns the currently active project.
// Returns nil and false if no project is active.
func (m *Manager) GetActive() (*Project, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.projects {
		if p.Active {
			copy := *p
			return &copy, true
		}
	}

	return nil, false
}

// SaveProjects persists the current project configurations to projects.yaml.
func (m *Manager) SaveProjects() error {
	m.mu.RLock()
	projects := m.ListProjectsLocked()
	m.mu.RUnlock()

	pf := projectsFile{
		Projects: projects,
	}

	data, err := yaml.Marshal(&pf)
	if err != nil {
		return fmt.Errorf("failed to serialize projects: %w", err)
	}

	// Ensure the config directory exists.
	if err := os.MkdirAll(m.configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	filePath := m.projectsFilePath()
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write projects file: %w", err)
	}

	return nil
}

// ListProjectsLocked returns all projects sorted by name.
// Caller must hold at least a read lock.
func (m *Manager) ListProjectsLocked() []Project {
	result := make([]Project, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, *p)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
