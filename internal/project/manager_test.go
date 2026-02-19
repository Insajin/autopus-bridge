package project

import (
	"os"
	"path/filepath"
	"testing"
)

// setupTestManager creates a Manager backed by a temporary directory.
func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()
	return NewManager(tmpDir)
}

// sampleProject returns a sample project for testing.
func sampleProject(name string) Project {
	return Project{
		Name:      name,
		Path:      "/home/user/projects/" + name,
		Workspace: "ws-" + name,
		Provider:  "claude",
		Active:    false,
	}
}

func TestNewManager(t *testing.T) {
	m := NewManager("/tmp/test-config")
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.configDir != "/tmp/test-config" {
		t.Errorf("configDir = %q, want %q", m.configDir, "/tmp/test-config")
	}
	if len(m.projects) != 0 {
		t.Errorf("projects should be empty, got %d", len(m.projects))
	}
}

func TestLoadProjects_FileNotExist(t *testing.T) {
	m := setupTestManager(t)
	err := m.LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects() on missing file should not error, got: %v", err)
	}
	if len(m.projects) != 0 {
		t.Errorf("projects should be empty when file missing, got %d", len(m.projects))
	}
}

func TestLoadProjects_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "projects.yaml")
	// Use YAML with a mapping where a sequence is expected to force a type mismatch.
	invalidYAML := "projects:\n  name: not-a-list\n  broken: [[[{"
	if err := os.WriteFile(filePath, []byte(invalidYAML), 0600); err != nil {
		t.Fatal(err)
	}

	m := NewManager(tmpDir)
	err := m.LoadProjects()
	if err == nil {
		t.Fatal("LoadProjects() should error on invalid YAML")
	}
}

func TestAddProject(t *testing.T) {
	m := setupTestManager(t)

	p := sampleProject("my-app")
	if err := m.AddProject(p); err != nil {
		t.Fatalf("AddProject() error: %v", err)
	}

	got, ok := m.GetProject("my-app")
	if !ok {
		t.Fatal("GetProject() returned false after AddProject()")
	}
	if got.Name != "my-app" {
		t.Errorf("Name = %q, want %q", got.Name, "my-app")
	}
	if got.Path != "/home/user/projects/my-app" {
		t.Errorf("Path = %q, want %q", got.Path, "/home/user/projects/my-app")
	}
	if got.Workspace != "ws-my-app" {
		t.Errorf("Workspace = %q, want %q", got.Workspace, "ws-my-app")
	}
	if got.Provider != "claude" {
		t.Errorf("Provider = %q, want %q", got.Provider, "claude")
	}
}

func TestAddProject_EmptyName(t *testing.T) {
	m := setupTestManager(t)
	p := Project{Name: "", Path: "/some/path"}
	if err := m.AddProject(p); err == nil {
		t.Fatal("AddProject() should error on empty name")
	}
}

func TestAddProject_EmptyPath(t *testing.T) {
	m := setupTestManager(t)
	p := Project{Name: "test", Path: ""}
	if err := m.AddProject(p); err == nil {
		t.Fatal("AddProject() should error on empty path")
	}
}

func TestAddProject_Duplicate(t *testing.T) {
	m := setupTestManager(t)

	p := sampleProject("dup-app")
	if err := m.AddProject(p); err != nil {
		t.Fatalf("first AddProject() error: %v", err)
	}

	if err := m.AddProject(p); err == nil {
		t.Fatal("AddProject() should error on duplicate name")
	}
}

func TestRemoveProject(t *testing.T) {
	m := setupTestManager(t)

	p := sampleProject("remove-me")
	_ = m.AddProject(p)

	if err := m.RemoveProject("remove-me"); err != nil {
		t.Fatalf("RemoveProject() error: %v", err)
	}

	_, ok := m.GetProject("remove-me")
	if ok {
		t.Fatal("GetProject() should return false after RemoveProject()")
	}
}

func TestRemoveProject_NotFound(t *testing.T) {
	m := setupTestManager(t)
	if err := m.RemoveProject("nonexistent"); err == nil {
		t.Fatal("RemoveProject() should error on missing project")
	}
}

func TestGetProject_NotFound(t *testing.T) {
	m := setupTestManager(t)
	_, ok := m.GetProject("ghost")
	if ok {
		t.Fatal("GetProject() should return false for missing project")
	}
}

func TestListProjects(t *testing.T) {
	m := setupTestManager(t)

	_ = m.AddProject(sampleProject("charlie"))
	_ = m.AddProject(sampleProject("alpha"))
	_ = m.AddProject(sampleProject("bravo"))

	list := m.ListProjects()
	if len(list) != 3 {
		t.Fatalf("ListProjects() returned %d projects, want 3", len(list))
	}

	// Verify alphabetical order.
	expected := []string{"alpha", "bravo", "charlie"}
	for i, name := range expected {
		if list[i].Name != name {
			t.Errorf("ListProjects()[%d].Name = %q, want %q", i, list[i].Name, name)
		}
	}
}

func TestListProjects_Empty(t *testing.T) {
	m := setupTestManager(t)
	list := m.ListProjects()
	if len(list) != 0 {
		t.Fatalf("ListProjects() returned %d projects, want 0", len(list))
	}
}

func TestSetActive(t *testing.T) {
	m := setupTestManager(t)

	_ = m.AddProject(sampleProject("proj-a"))
	_ = m.AddProject(sampleProject("proj-b"))

	// Activate proj-a.
	if err := m.SetActive("proj-a"); err != nil {
		t.Fatalf("SetActive(proj-a) error: %v", err)
	}

	active, ok := m.GetActive()
	if !ok {
		t.Fatal("GetActive() should return true after SetActive()")
	}
	if active.Name != "proj-a" {
		t.Errorf("active project = %q, want %q", active.Name, "proj-a")
	}

	// Switch to proj-b. proj-a should be deactivated.
	if err := m.SetActive("proj-b"); err != nil {
		t.Fatalf("SetActive(proj-b) error: %v", err)
	}

	active, ok = m.GetActive()
	if !ok {
		t.Fatal("GetActive() should return true after switching")
	}
	if active.Name != "proj-b" {
		t.Errorf("active project = %q, want %q", active.Name, "proj-b")
	}

	// Verify proj-a is no longer active.
	projA, _ := m.GetProject("proj-a")
	if projA.Active {
		t.Error("proj-a should not be active after switching to proj-b")
	}
}

func TestSetActive_NotFound(t *testing.T) {
	m := setupTestManager(t)
	if err := m.SetActive("missing"); err == nil {
		t.Fatal("SetActive() should error on missing project")
	}
}

func TestGetActive_NoneActive(t *testing.T) {
	m := setupTestManager(t)
	_ = m.AddProject(sampleProject("inactive-proj"))

	_, ok := m.GetActive()
	if ok {
		t.Fatal("GetActive() should return false when no project is active")
	}
}

func TestSaveAndLoadProjects(t *testing.T) {
	tmpDir := t.TempDir()

	// Create and save projects.
	m1 := NewManager(tmpDir)
	_ = m1.AddProject(Project{
		Name:      "web-app",
		Path:      "/home/user/web-app",
		Workspace: "ws-web",
		Provider:  "claude",
		Active:    true,
	})
	_ = m1.AddProject(Project{
		Name:      "api-server",
		Path:      "/home/user/api-server",
		Workspace: "ws-api",
		Provider:  "gemini",
		Active:    false,
	})

	if err := m1.SaveProjects(); err != nil {
		t.Fatalf("SaveProjects() error: %v", err)
	}

	// Verify file exists.
	filePath := filepath.Join(tmpDir, "projects.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("projects.yaml was not created")
	}

	// Load into a new manager.
	m2 := NewManager(tmpDir)
	if err := m2.LoadProjects(); err != nil {
		t.Fatalf("LoadProjects() error: %v", err)
	}

	list := m2.ListProjects()
	if len(list) != 2 {
		t.Fatalf("loaded %d projects, want 2", len(list))
	}

	// Verify project data survived round-trip.
	webApp, ok := m2.GetProject("web-app")
	if !ok {
		t.Fatal("web-app project not found after load")
	}
	if webApp.Path != "/home/user/web-app" {
		t.Errorf("web-app.Path = %q, want %q", webApp.Path, "/home/user/web-app")
	}
	if webApp.Workspace != "ws-web" {
		t.Errorf("web-app.Workspace = %q, want %q", webApp.Workspace, "ws-web")
	}
	if webApp.Provider != "claude" {
		t.Errorf("web-app.Provider = %q, want %q", webApp.Provider, "claude")
	}
	if !webApp.Active {
		t.Error("web-app should be active after load")
	}

	apiServer, ok := m2.GetProject("api-server")
	if !ok {
		t.Fatal("api-server project not found after load")
	}
	if apiServer.Active {
		t.Error("api-server should not be active after load")
	}
}

func TestSaveProjects_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "deep", "config")

	m := NewManager(nestedDir)
	_ = m.AddProject(sampleProject("test"))

	if err := m.SaveProjects(); err != nil {
		t.Fatalf("SaveProjects() should create nested dirs, got error: %v", err)
	}

	filePath := filepath.Join(nestedDir, "projects.yaml")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("projects.yaml was not created in nested directory")
	}
}

func TestSaveProjects_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir)
	_ = m.AddProject(sampleProject("secure-proj"))

	if err := m.SaveProjects(); err != nil {
		t.Fatalf("SaveProjects() error: %v", err)
	}

	filePath := filepath.Join(tmpDir, "projects.yaml")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestGetProject_ReturnsCopy(t *testing.T) {
	m := setupTestManager(t)
	_ = m.AddProject(sampleProject("original"))

	got, _ := m.GetProject("original")
	got.Name = "mutated"

	// The mutation should not affect the stored project.
	stored, _ := m.GetProject("original")
	if stored.Name != "original" {
		t.Error("GetProject() should return a copy; mutation leaked to stored project")
	}
}
