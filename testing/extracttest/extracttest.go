// Package extracttest provides structures to help create tabular tests for extractors.
package extracttest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/osv-scalibr/extractor"
	"github.com/google/osv-scalibr/extractor/filesystem"
	scalibrfs "github.com/google/osv-scalibr/fs"
	"github.com/google/osv-scalibr/testing/fakefs"
	"github.com/google/osv-scalibr/testing/internal/inventorysorter"
)

// ScanInputMockConfig is used to quickly configure building a mock ScanInput
type ScanInputMockConfig struct {
	// Path of the file ScanInput will read, relative to the ScanRoot
	Path string
	// FakeScanRoot allows you to set a custom scanRoot, can be relative or absolute,
	// and will be translated to an absolute path
	FakeScanRoot string
	FakeFileInfo *fakefs.FakeFileInfo
}

type TestTableEntry struct {
	Name              string
	InputConfig       ScanInputMockConfig
	WantInventory     []*extractor.Inventory
	WantErrIs         error
	WantErrContaining string
}

// ExtractionTester tests common properties of a extractor, and returns the raw values from running extract
func ExtractionTester(t *testing.T, extractor filesystem.Extractor, tt TestTableEntry) ([]*extractor.Inventory, error) {
	t.Helper()

	wrapper := generateScanInputMock(t, tt.InputConfig)
	got, err := extractor.Extract(context.Background(), &wrapper.ScanInput)
	wrapper.close()

	// Check if expected errors match
	if tt.WantErrContaining == "" && tt.WantErrIs == nil {
		if err != nil {
			t.Errorf("Got error when expecting none: '%s'", err)
			return got, err
		}
	} else {
		if err == nil {
			t.Errorf("Expected to get error, but did not.")
			return got, err
		}
	}

	if tt.WantErrIs != nil {
		if !errors.Is(err, tt.WantErrIs) {
			t.Errorf("Expected to get \"%v\" error but got \"%v\" instead", tt.WantErrIs, err)
		}
		return got, err
	}

	if tt.WantErrContaining != "" {
		if !strings.Contains(err.Error(), tt.WantErrContaining) {
			t.Errorf("Expected to get \"%s\" error, but got \"%v\"", tt.WantErrContaining, err)
		}
		return got, err
	}

	// Check if result match if no errors
	inventorysorter.Sort(got)
	inventorysorter.Sort(tt.WantInventory)
	if !cmp.Equal(got, tt.WantInventory) {
		t.Errorf("%s.Extract(%s) diff: \n%s", extractor.Name(), tt.InputConfig.Path, cmp.Diff(got, tt.WantInventory))
	}

	return got, err
}

type scanInputWrapper struct {
	fileHandle *os.File
	ScanInput  filesystem.ScanInput
}

func (siw scanInputWrapper) close() {
	siw.fileHandle.Close()
}

// generateScanInputMock will try to open the file locally, and fail if the file doesn't exist
func generateScanInputMock(t *testing.T, config ScanInputMockConfig) scanInputWrapper {
	t.Helper()

	var scanRoot string
	if filepath.IsAbs(config.FakeScanRoot) {
		scanRoot = config.FakeScanRoot
	} else {
		workingDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Can't get working directory because '%s'", workingDir)
		}
		scanRoot = filepath.Join(workingDir, config.FakeScanRoot)
	}

	f, err := os.Open(filepath.Join(scanRoot, config.Path))
	if err != nil {
		t.Fatalf("Can't open test fixture '%s' because '%s'", config.Path, err)
	}
	info, err := f.Stat()
	if err != nil {
		t.Fatalf("Can't stat test fixture '%s' because '%s'", config.Path, err)
	}

	return scanInputWrapper{
		fileHandle: f,
		ScanInput: filesystem.ScanInput{
			FS:     os.DirFS(scanRoot).(scalibrfs.FS),
			Path:   config.Path,
			Root:   scanRoot,
			Reader: f,
			Info:   info,
		},
	}
}
