package archiver

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Action is what happened to a file.
type Action string

const (
	ActCopied    Action = "copied"
	ActDuplicate Action = "duplicate"
	ActError     Action = "error"
	ActPlanned   Action = "planned" // dry run
)

// FileResult is the outcome for one source file.
type FileResult struct {
	Src        string `json:"src"`
	Dst        string `json:"dst,omitempty"`
	Action     Action `json:"action"`
	Category   string `json:"category,omitempty"`
	Year       int    `json:"year,omitempty"`
	DateSource string `json:"dateSource,omitempty"`
	Method     string `json:"method,omitempty"`
	Renamed    bool   `json:"renamed,omitempty"`
	DupOf      string `json:"dupOf,omitempty"`
	Err        string `json:"err,omitempty"`
}

// Run statuses.
const (
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusDiskError = "disk_error"
	StatusError     = "error"
)

// Report summarises a run.
type Report struct {
	RunID      int64        `json:"runId"`
	Root       string       `json:"root"`
	Sources    []string     `json:"sources"`
	DryRun     bool         `json:"dryRun"`
	Started    time.Time    `json:"started"`
	Finished   time.Time    `json:"finished"`
	Status     string       `json:"status"`
	Message    string       `json:"message,omitempty"`
	Stats      Progress     `json:"stats"`
	Renamed    []FileResult `json:"renamed"`
	Errors     []FileResult `json:"errors"`
	NoDate     []FileResult `json:"noDate"`
	MtimeDates []FileResult `json:"mtimeDates"`
	Planned    []FileResult `json:"planned,omitempty"`
	ReportPath string       `json:"reportPath"`
	CSVPath    string       `json:"csvPath"`
}

// maxListed caps per-list entries kept in memory/JSON; the CSV has all rows.
const maxListed = 5000

// collector gathers file results concurrently and streams them to CSV.
type collector struct {
	mu   sync.Mutex
	rep  *Report
	csvF *os.File
	csvW *csv.Writer
}

func newCollector(rep *Report, csvPath string) *collector {
	c := &collector{rep: rep}
	if csvPath == "" {
		return c
	}
	if err := os.MkdirAll(filepath.Dir(csvPath), 0o755); err != nil {
		return c
	}
	f, err := os.Create(csvPath)
	if err != nil {
		return c
	}
	f.WriteString("\ufeff") // BOM so Excel opens UTF-8 correctly
	c.csvF, c.csvW = f, csv.NewWriter(f)
	c.csvW.Comma = ';'
	_ = c.csvW.Write([]string{"action", "src", "dst", "category", "year", "date_source", "method", "renamed", "duplicate_of", "error"})
	rep.CSVPath = csvPath
	return c
}

func appendCapped(list []FileResult, r FileResult) []FileResult {
	if len(list) >= maxListed {
		return list
	}
	return append(list, r)
}

func (c *collector) add(r FileResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch {
	case r.Action == ActError:
		c.rep.Errors = appendCapped(c.rep.Errors, r)
	case r.Action == ActPlanned:
		c.rep.Planned = appendCapped(c.rep.Planned, r)
	}
	if r.Renamed {
		c.rep.Renamed = appendCapped(c.rep.Renamed, r)
	}
	if r.Action == ActCopied || r.Action == ActPlanned {
		switch r.DateSource {
		case "none":
			c.rep.NoDate = appendCapped(c.rep.NoDate, r)
		case "mtime":
			c.rep.MtimeDates = appendCapped(c.rep.MtimeDates, r)
		}
	}
	if c.csvW != nil {
		year := ""
		if r.Year > 0 {
			year = strconv.Itoa(r.Year)
		}
		_ = c.csvW.Write([]string{string(r.Action), r.Src, r.Dst, r.Category, year, r.DateSource,
			r.Method, strconv.FormatBool(r.Renamed), r.DupOf, r.Err})
	}
}

func (c *collector) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.csvW != nil {
		c.csvW.Flush()
		c.csvF.Close()
		c.csvW = nil
	}
}

// saveJSON writes the report next to the CSV.
func saveJSON(rep *Report, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
