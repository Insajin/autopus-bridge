package ws

// ProjectContextPayload is sent by the bridge after analyzing the user's project.
// Message type: project_context (Bridge -> Server)
type ProjectContextPayload struct {
	ProjectRoot string    `json:"project_root"`
	TechStack   TechStack `json:"tech_stack"`
	DetectedAt  string    `json:"detected_at"`
}

// TechStack describes the technology stack detected in a user's project.
type TechStack struct {
	Languages      []string `json:"languages"`
	Frameworks     []string `json:"frameworks"`
	Databases      []string `json:"databases"`
	BuildTools     []string `json:"build_tools"`
	TestFrameworks []string `json:"test_frameworks"`
	DetectedFiles  []string `json:"detected_files"`
}

// CLIRequestPayload is sent by the server to request CLI execution on the bridge.
// Message type: cli_request (Server -> Bridge)
type CLIRequestPayload struct {
	Command        string            `json:"command"`
	WorkingDir     string            `json:"working_dir"`
	TimeoutSeconds int               `json:"timeout_seconds"`
	Env            map[string]string `json:"env,omitempty"`
	ParseFormat    string            `json:"parse_format"` // plain_text, json, go_test_json, tap, junit_xml
}

// CLIResultPayload is sent by the bridge after CLI execution completes.
// Message type: cli_result (Bridge -> Server)
type CLIResultPayload struct {
	ExitCode        int              `json:"exit_code"`
	Stdout          string           `json:"stdout,omitempty"`
	Stderr          string           `json:"stderr,omitempty"`
	ParsedResult    *CLIParsedResult `json:"parsed_result,omitempty"`
	DurationMs      int64            `json:"duration_ms"`
	StdoutTruncated bool             `json:"stdout_truncated"`
	StderrTruncated bool             `json:"stderr_truncated"`
}

// CLIParsedResult contains structured output from CLI command parsing.
type CLIParsedResult struct {
	Total    int              `json:"total,omitempty"`
	Passed   int              `json:"passed,omitempty"`
	Failed   int              `json:"failed,omitempty"`
	Skipped  int              `json:"skipped,omitempty"`
	Failures []CLITestFailure `json:"failures,omitempty"`
	Summary  string           `json:"summary,omitempty"`
}

// CLITestFailure describes a single test failure from CLI output parsing.
type CLITestFailure struct {
	Test    string `json:"test"`
	Package string `json:"package,omitempty"`
	Output  string `json:"output"`
}
