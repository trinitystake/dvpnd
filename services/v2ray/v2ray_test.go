// SPDX-License-Identifier: Apache-2.0

package v2ray

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// stub writes an executable script named v2ray into a temp dir and points
// binaryName at it. The script's body decides how it reacts to SIGTERM.
// It returns the path of a file the script creates once its trap is
// installed, so a test can wait for the child to be ready before signalling.
func stub(t *testing.T, body string) string {
	t.Helper()

	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	path := filepath.Join(dir, "v2ray")
	script := "#!/bin/sh\n" + body + "\ntouch " + ready + "\nwhile :; do sleep 0.1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	saved := binaryName
	binaryName = path
	t.Cleanup(func() { binaryName = saved })

	return ready
}

func startStub(t *testing.T, body string) *V2Ray {
	t.Helper()

	ready := stub(t, body)
	s := NewV2Ray()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatal("stub never became ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestInitRequiresBinaryOnPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	saved := binaryName
	binaryName = "v2ray"
	t.Cleanup(func() { binaryName = saved })

	err := NewV2Ray().Init(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `"v2ray" binary is not on PATH`) {
		t.Fatalf("Init without the binary: %v", err)
	}
}

func TestStopTerminatesChild(t *testing.T) {
	s := startStub(t, "trap 'exit 0' TERM")

	start := time.Now()
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("Stop took %s, the child should have exited on SIGTERM", time.Since(start))
	}
	if s.cmd.ProcessState == nil || !s.cmd.ProcessState.Exited() {
		t.Fatal("child was not reaped")
	}
}

func TestStopKillsChildThatIgnoresTerm(t *testing.T) {
	saved := stopTimeout
	stopTimeout = 300 * time.Millisecond
	t.Cleanup(func() { stopTimeout = saved })

	s := startStub(t, "trap '' TERM")
	if err := s.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if s.cmd.ProcessState == nil || s.cmd.ProcessState.Success() {
		t.Fatal("child should have been killed")
	}
}

func TestStopWithoutStart(t *testing.T) {
	if err := NewV2Ray().Stop(); err == nil {
		t.Fatal("Stop before Start must fail")
	}
}
