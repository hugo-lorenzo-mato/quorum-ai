package cmd

import "testing"

func TestSnapshotCommand_Structure(t *testing.T) {
	var found *bool
	for _, c := range rootCmd.Commands() {
		if c.Use == "snapshot" {
			v := true
			found = &v
			break
		}
	}
	if found == nil || !*found {
		t.Fatalf("snapshot command not registered")
	}

	if snapshotCmd.Commands() == nil || len(snapshotCmd.Commands()) < 3 {
		t.Fatalf("expected snapshot subcommands export/import/validate")
	}

	if snapshotExportCmd.Flags().Lookup("output") == nil {
		t.Fatalf("snapshot export missing --output flag")
	}
	if snapshotImportCmd.Flags().Lookup("input") == nil {
		t.Fatalf("snapshot import missing --input flag")
	}
	if snapshotValidateCmd.Flags().Lookup("input") == nil {
		t.Fatalf("snapshot validate missing --input flag")
	}
}

func TestParsePathMapFlags(t *testing.T) {
	m, err := parsePathMapFlags([]string{"/a=/b", "/c=/d"})
	if err != nil {
		t.Fatalf("parsePathMapFlags() error = %v", err)
	}
	if m["/a"] != "/b" || m["/c"] != "/d" {
		t.Fatalf("unexpected map contents: %#v", m)
	}

	if _, err := parsePathMapFlags([]string{"invalid"}); err == nil {
		t.Fatalf("expected parsePathMapFlags to fail for invalid value")
	}
}
