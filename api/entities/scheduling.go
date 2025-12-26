package entities

// SchedulingImportError represents a single cell-level error during scheduling import.
type SchedulingImportError struct {
	SatpamName string `json:"satpam_name"`
	Date       string `json:"date"`
	Value      string `json:"value"`
	Reason     string `json:"reason"`
}

// SchedulingImportResult summarizes the outcome of an import run.
type SchedulingImportResult struct {
	ProcessedRows  int                     `json:"processed_rows"`
	ProcessedCells int                     `json:"processed_cells"`
	Inserted       int                     `json:"inserted"`
	Updated        int                     `json:"updated"`
	Skipped        int                     `json:"skipped"`
	Errors         []SchedulingImportError `json:"errors"`
}

