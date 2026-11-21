package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/snmp161/melisa/internal/model"
)

const reportSchemaDraft = "https://json-schema.org/draft/2020-12/schema"

type reportSchemaDocument struct {
	ID     string `json:"$id"`
	Schema string `json:"$schema"`
	Defs   struct {
		Metadata struct {
			Properties struct {
				SchemaVersion struct {
					Const string `json:"const"`
				} `json:"schema_version"`
			} `json:"properties"`
		} `json:"Metadata"`
	} `json:"$defs"`
}

func TestReportSchemaVersion(t *testing.T) {
	_, document := loadReportSchema(t)
	if document.Schema != reportSchemaDraft {
		t.Errorf("schema draft = %q, want %q", document.Schema, reportSchemaDraft)
	}
	if document.ID == "" {
		t.Error("schema $id is empty")
	}
	got := document.Defs.Metadata.Properties.SchemaVersion.Const
	if got != model.SchemaVersion {
		t.Errorf("schema_version const = %q, want %q", got, model.SchemaVersion)
	}
}

func TestCapturedReportMatchesSchema(t *testing.T) {
	schema, _ := loadReportSchema(t)
	path := filepath.Join("..", "..", "testdata", "report_"+model.SchemaVersion+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read captured report: %v", err)
	}

	var report model.Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode captured report into model: %v", err)
	}
	if report.Metadata.SchemaVersion != model.SchemaVersion {
		t.Errorf("captured schema_version = %q, want %q", report.Metadata.SchemaVersion, model.SchemaVersion)
	}

	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode captured report as JSON: %v", err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("captured report does not match schema: %v", err)
	}
}

func TestWriteJSONMatchesSchema(t *testing.T) {
	start := time.Date(2024, 1, 15, 10, 29, 30, 0, time.UTC)
	end := start.Add(30 * time.Second)
	report := &model.Report{
		Metadata: model.Metadata{
			Tool:          "melisa",
			Version:       "test",
			SchemaVersion: model.SchemaVersion,
			Hostname:      "example-host",
			Timestamp:     end.Format(time.RFC3339),
			Duration:      "30s",
			Profile:       "quick",
			FocusAreas:    nil,
			KernelVersion: "6.8.0-example",
			Arch:          "amd64",
			CPUs:          1,
			Capabilities:  nil,
			ContainerEnv:  "none",
			CgroupVersion: 2,
		},
		System: model.SystemInfo{
			OS:     "Example Linux",
			Kernel: "6.8.0-example",
		},
		Categories: map[string][]model.Result{
			"cpu": {
				{
					Collector: "cpu_utilization",
					Category:  "cpu",
					Tier:      1,
					StartTime: start,
					EndTime:   end,
					Data:      &model.CPUData{NumCPUs: 1, IdlePct: 100},
				},
			},
			"network": {
				{
					Collector: "tcpretrans",
					Category:  "network",
					Tier:      2,
					StartTime: start,
					EndTime:   end,
					Data:      nil,
				},
			},
			"stacktrace": {
				{
					Collector: "profile",
					Category:  "stacktrace",
					Tier:      2,
					StartTime: start,
					EndTime:   end,
					Data:      &model.StackData{},
				},
			},
		},
		Summary: model.Summary{
			HealthScore: 100,
			Anomalies:   nil,
			Resources: map[string]model.USEMetric{
				"cpu": {Utilization: 0},
			},
		},
		AIContext: &model.AIContext{
			Prompt:        "Analyze this test report.",
			Methodology:   "USE",
			KnownPatterns: []string{},
		},
	}

	path := filepath.Join(t.TempDir(), "report.json")
	if err := WriteJSON(report, path); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read generated report: %v", err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode generated report as JSON: %v", err)
	}
	schema, _ := loadReportSchema(t)
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("generated report does not match schema: %v", err)
	}
}

func TestReportSchemaRejectsInvalidDocuments(t *testing.T) {
	schema, _ := loadReportSchema(t)
	path := filepath.Join("..", "..", "testdata", "report_"+model.SchemaVersion+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read captured report: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{
			name: "missing required top-level field",
			mutate: func(document map[string]any) {
				delete(document, "summary")
			},
		},
		{
			name: "wrong field type",
			mutate: func(document map[string]any) {
				document["summary"].(map[string]any)["health_score"] = "high"
			},
		},
		{
			name: "unsupported schema version",
			mutate: func(document map[string]any) {
				document["metadata"].(map[string]any)["schema_version"] = "9.9.9"
			},
		},
		{
			name: "invalid timestamp",
			mutate: func(document map[string]any) {
				document["metadata"].(map[string]any)["timestamp"] = "not-a-time"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("decode test document: %v", err)
			}
			document := instance.(map[string]any)
			tt.mutate(document)
			if err := schema.Validate(document); err == nil {
				t.Fatal("invalid document unexpectedly passed schema validation")
			}
		})
	}
}

func loadReportSchema(t *testing.T) (*jsonschema.Schema, reportSchemaDocument) {
	t.Helper()
	path := filepath.Join("..", "..", "docs", "schema", "report-"+model.SchemaVersion+".schema.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report schema: %v", err)
	}

	var document reportSchemaDocument
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("decode schema metadata: %v", err)
	}
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode schema resource: %v", err)
	}
	if document.ID == "" {
		t.Fatal("schema $id is empty")
	}

	compiler := jsonschema.NewCompiler()
	compiler.AssertFormat()
	if err := compiler.AddResource(document.ID, resource); err != nil {
		t.Fatalf("add schema resource: %v", err)
	}
	schema, err := compiler.Compile(document.ID)
	if err != nil {
		t.Fatalf("compile report schema: %v", err)
	}
	return schema, document
}
