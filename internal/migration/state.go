package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StepStatus represents the status of a migration step
type StepStatus string

const (
	StatusPending    StepStatus = "pending"
	StatusInProgress StepStatus = "in_progress"
	StatusCompleted  StepStatus = "completed"
	StatusFailed     StepStatus = "failed"
	StatusSkipped    StepStatus = "skipped"
)

// StepName represents the name of a migration step
type StepName string

const (
	StepExportAssets       StepName = "export_assets"
	StepImportAssets       StepName = "import_assets"
	StepExportMemberships  StepName = "export_memberships"
	StepImportMemberships  StepName = "import_memberships"
	StepExportMessages     StepName = "export_messages"
	StepImportMessages     StepName = "import_messages"
)

// StepState represents the state of a single migration step
type StepState struct {
	Name           StepName   `json:"name"`
	Status         StepStatus `json:"status"`
	StartedAt      int64      `json:"started_at,omitempty"`
	CompletedAt    int64      `json:"completed_at,omitempty"`
	ItemsProcessed int        `json:"items_processed,omitempty"`
	ItemsTotal     int        `json:"items_total,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	OutputFile     string     `json:"output_file,omitempty"`
}

// MigrationState represents the overall migration state
type MigrationState struct {
	Version       string                `json:"version"`
	CreatedAt     int64                 `json:"created_at"`
	UpdatedAt     int64                 `json:"updated_at"`
	MattermostHost string               `json:"mattermost_host,omitempty"`
	MatrixHost    string                `json:"matrix_host,omitempty"`
	Steps         map[StepName]*StepState `json:"steps"`
}

// NewMigrationState creates a new migration state
func NewMigrationState() *MigrationState {
	now := time.Now().UnixMilli()
	return &MigrationState{
		Version:   "1.0",
		CreatedAt: now,
		UpdatedAt: now,
		Steps:     make(map[StepName]*StepState),
	}
}

// GetStep gets or creates a step state
func (s *MigrationState) GetStep(name StepName) *StepState {
	if step, exists := s.Steps[name]; exists {
		return step
	}
	step := &StepState{
		Name:   name,
		Status: StatusPending,
	}
	s.Steps[name] = step
	return step
}

// StartStep marks a step as started
func (s *MigrationState) StartStep(name StepName) *StepState {
	step := s.GetStep(name)
	step.Status = StatusInProgress
	step.StartedAt = time.Now().UnixMilli()
	step.ErrorMessage = ""
	s.UpdatedAt = time.Now().UnixMilli()
	return step
}

// UpdateStepProgress updates the progress of a step
func (s *MigrationState) UpdateStepProgress(name StepName, processed, total int) {
	step := s.GetStep(name)
	step.ItemsProcessed = processed
	step.ItemsTotal = total
	s.UpdatedAt = time.Now().UnixMilli()
}

// CompleteStep marks a step as completed
func (s *MigrationState) CompleteStep(name StepName, outputFile string) {
	step := s.GetStep(name)
	step.Status = StatusCompleted
	step.CompletedAt = time.Now().UnixMilli()
	step.OutputFile = outputFile
	s.UpdatedAt = time.Now().UnixMilli()
}

// FailStep marks a step as failed
func (s *MigrationState) FailStep(name StepName, err error) {
	step := s.GetStep(name)
	step.Status = StatusFailed
	step.CompletedAt = time.Now().UnixMilli()
	step.ErrorMessage = err.Error()
	s.UpdatedAt = time.Now().UnixMilli()
}

// SkipStep marks a step as skipped
func (s *MigrationState) SkipStep(name StepName, reason string) {
	step := s.GetStep(name)
	step.Status = StatusSkipped
	step.CompletedAt = time.Now().UnixMilli()
	step.ErrorMessage = reason
	s.UpdatedAt = time.Now().UnixMilli()
}

// CanRunStep checks if a step can be run based on prerequisites
func (s *MigrationState) CanRunStep(name StepName) (bool, string) {
	switch name {
	case StepExportAssets:
		// Can always run
		return true, ""
	case StepImportAssets:
		// Requires export_assets to be completed
		exportStep := s.GetStep(StepExportAssets)
		if exportStep.Status != StatusCompleted {
			return false, "export_assets must be completed first"
		}
		return true, ""
	case StepExportMemberships:
		// Requires import_assets to be completed (for mapping file)
		importStep := s.GetStep(StepImportAssets)
		if importStep.Status != StatusCompleted {
			return false, "import_assets must be completed first"
		}
		return true, ""
	case StepImportMemberships:
		// Requires export_memberships to be completed
		exportStep := s.GetStep(StepExportMemberships)
		if exportStep.Status != StatusCompleted {
			return false, "export_memberships must be completed first"
		}
		return true, ""
	case StepExportMessages:
		// Requires export_assets to be completed (for channel list)
		exportStep := s.GetStep(StepExportAssets)
		if exportStep.Status != StatusCompleted {
			return false, "export_assets must be completed first"
		}
		return true, ""
	case StepImportMessages:
		// Requires export_messages and import_assets to be completed
		exportMsgStep := s.GetStep(StepExportMessages)
		if exportMsgStep.Status != StatusCompleted {
			return false, "export_messages must be completed first"
		}
		importAssetsStep := s.GetStep(StepImportAssets)
		if importAssetsStep.Status != StatusCompleted {
			return false, "import_assets must be completed first (for room and user mappings)"
		}
		return true, ""
	}
	return false, "unknown step"
}

// GetStepOutputFile returns the output file path for a step
func (s *MigrationState) GetStepOutputFile(name StepName) string {
	step := s.GetStep(name)
	return step.OutputFile
}

// IsComplete checks if all steps are completed
func (s *MigrationState) IsComplete() bool {
	requiredSteps := []StepName{
		StepExportAssets,
		StepImportAssets,
		StepExportMemberships,
		StepImportMemberships,
		StepExportMessages,
		StepImportMessages,
	}

	for _, name := range requiredSteps {
		step := s.GetStep(name)
		if step.Status != StatusCompleted && step.Status != StatusSkipped {
			return false
		}
	}
	return true
}

// Summary returns a summary of the migration state
type StateSummary struct {
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Completed  int `json:"completed"`
	Failed     int `json:"failed"`
	Skipped    int `json:"skipped"`
}

func (s *MigrationState) Summary() StateSummary {
	summary := StateSummary{}
	for _, step := range s.Steps {
		switch step.Status {
		case StatusPending:
			summary.Pending++
		case StatusInProgress:
			summary.InProgress++
		case StatusCompleted:
			summary.Completed++
		case StatusFailed:
			summary.Failed++
		case StatusSkipped:
			summary.Skipped++
		}
	}
	return summary
}

// SaveState saves the migration state to a JSON file
func SaveState(state *MigrationState, filePath string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// LoadState loads the migration state from a JSON file
func LoadState(filePath string) (*MigrationState, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return new state if file doesn't exist
			return NewMigrationState(), nil
		}
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// StateExists checks if a state file exists
func StateExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}




