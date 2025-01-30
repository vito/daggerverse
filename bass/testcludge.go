package main

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"regexp"
	"runtime/pprof"
	"time"
)

// Cover indicates whether coverage is enabled.
var Cover bool

// TestDeps is an implementation of the testing.testDeps interface,
// suitable for passing to [testing.MainStart].
type TestDeps struct{}

var matchPat string
var matchRe *regexp.Regexp

func (TestDeps) MatchString(pat, str string) (result bool, err error) {
	if matchRe == nil || matchPat != pat {
		matchPat = pat
		matchRe, err = regexp.Compile(matchPat)
		if err != nil {
			return
		}
	}
	return matchRe.MatchString(str), nil
}

func (TestDeps) StartCPUProfile(w io.Writer) error {
	return pprof.StartCPUProfile(w)
}

func (TestDeps) StopCPUProfile() {
	pprof.StopCPUProfile()
}

func (TestDeps) WriteProfileTo(name string, w io.Writer, debug int) error {
	return pprof.Lookup(name).WriteTo(w, debug)
}

// ImportPath is the import path of the testing binary, set by the generated main function.
var ImportPath string

func (TestDeps) ImportPath() string {
	return ImportPath
}

func (TestDeps) StartTestLog(w io.Writer) {}

func (TestDeps) StopTestLog() error {
	return nil
}

// SetPanicOnExit0 tells the os package whether to panic on os.Exit(0).
func (TestDeps) SetPanicOnExit0(v bool) {
}

func (TestDeps) CoordinateFuzzing(
	timeout time.Duration,
	limit int64,
	minimizeTimeout time.Duration,
	minimizeLimit int64,
	parallel int,
	seed []CorpusEntry,
	types []reflect.Type,
	corpusDir,
	cacheDir string) error {
	return fmt.Errorf("fuzzing not supported")
}

func (TestDeps) RunFuzzWorker(fn func(CorpusEntry) error) error {
	return fmt.Errorf("fuzzing not supported")
}

func (TestDeps) ReadCorpus(dir string, types []reflect.Type) ([]CorpusEntry, error) {
	return nil, fmt.Errorf("fuzzing not supported")
}

func (TestDeps) CheckCorpus(vals []any, types []reflect.Type) error {
	return fmt.Errorf("fuzzing not supported")
}

func (TestDeps) ResetCoverage() {
}

func (TestDeps) SnapshotCoverage() {
}

var CoverMode string
var Covered string
var CoverSelectedPackages []string

// These variables below are set at runtime (via code in testmain) to point
// to the equivalent functions in package internal/coverage/cfile; doing
// things this way allows us to have tests import internal/coverage/cfile
// only when -cover is in effect (as opposed to importing for all tests).
var (
	CoverSnapshotFunc           func() float64
	CoverProcessTestDirFunc     func(dir string, cfile string, cm string, cpkg string, w io.Writer, selpkgs []string) error
	CoverMarkProfileEmittedFunc func(val bool)
)

func (TestDeps) InitRuntimeCoverage() (mode string, tearDown func(string, string) (string, error), snapcov func() float64) {
	if CoverMode == "" {
		return
	}
	return CoverMode, coverTearDown, CoverSnapshotFunc
}

func coverTearDown(coverprofile string, gocoverdir string) (string, error) {
	var err error
	if gocoverdir == "" {
		gocoverdir, err = os.MkdirTemp("", "gocoverdir")
		if err != nil {
			return "error setting GOCOVERDIR: bad os.MkdirTemp return", err
		}
		defer os.RemoveAll(gocoverdir)
	}
	CoverMarkProfileEmittedFunc(true)
	cmode := CoverMode
	if err := CoverProcessTestDirFunc(gocoverdir, coverprofile, cmode, Covered, os.Stdout, CoverSelectedPackages); err != nil {
		return "error generating coverage report", err
	}
	return "", nil
}

// CorpusEntry represents an individual input for fuzzing.
//
// We must use an equivalent type in the testing and testing/internal/testdeps
// packages, but testing can't import this package directly, and we don't want
// to export this type from testing. Instead, we use the same struct type and
// use a type alias (not a defined type) for convenience.
type CorpusEntry = struct {
	Parent string

	// Path is the path of the corpus file, if the entry was loaded from disk.
	// For other entries, including seed values provided by f.Add, Path is the
	// name of the test, e.g. seed#0 or its hash.
	Path string

	// Data is the raw input data. Data should only be populated for seed
	// values. For on-disk corpus files, Data will be nil, as it will be loaded
	// from disk using Path.
	Data []byte

	// Values is the unmarshaled values from a corpus file.
	Values []any

	Generation int

	// IsSeed indicates whether this entry is part of the seed corpus.
	IsSeed bool
}
