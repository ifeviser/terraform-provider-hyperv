package provider

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestComputeFileSHA256(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.iso")

	content := []byte("hello iso content")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	got, err := computeFileSHA256(path)
	if err != nil {
		t.Fatalf("computeFileSHA256 returned error: %v", err)
	}

	if len(got) != hex.EncodedLen(32) {
		t.Errorf("expected 64-char hex digest, got %d chars: %s", len(got), got)
	}

	// Same content should produce the same hash.
	got2, err := computeFileSHA256(path)
	if err != nil {
		t.Fatalf("second computeFileSHA256 returned error: %v", err)
	}
	if got != got2 {
		t.Errorf("hash is non-deterministic: %s != %s", got, got2)
	}

	// Different content should produce a different hash.
	if err := os.WriteFile(path, []byte("different content"), 0600); err != nil {
		t.Fatalf("failed to overwrite temp file: %v", err)
	}
	got3, err := computeFileSHA256(path)
	if err != nil {
		t.Fatalf("computeFileSHA256 on updated file returned error: %v", err)
	}
	if got == got3 {
		t.Errorf("expected different hash after file update, got same: %s", got)
	}
}

func TestComputeFileSHA256_NonExistent(t *testing.T) {
	t.Parallel()

	_, err := computeFileSHA256("/nonexistent/path/file.iso")
	if err == nil {
		t.Error("expected an error for non-existent file, got nil")
	}
}

func TestHyperVIsoImageCustomizeDiff_DoesNotPlanAutoComputedZipHashOnCreate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "artifact.zip")
	if err := os.WriteFile(zipPath, []byte("zip-content-v1"), 0o600); err != nil {
		t.Fatalf("failed to write initial zip artifact: %v", err)
	}

	isoPath := filepath.Join(dir, "output.iso")
	resource := resourceHyperVIsoImage()
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"source_zip_file_path":      zipPath,
		"destination_iso_file_path": isoPath,
	})

	createDiff, err := resource.Diff(context.Background(), nil, config, nil)
	if err != nil {
		t.Fatalf("create diff failed: %v", err)
	}

	if attr := createDiff.Attributes["source_zip_file_path_hash"]; attr != nil && !attr.NewComputed && attr.New != "" {
		t.Fatalf("expected create plan to avoid pinning a concrete zip hash, got %#v", attr)
	}
}

func TestHyperVIsoImageCustomizeDiff_PreservesPlannedZipHashWhenSourceChangesBetweenPlans(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "artifact.zip")
	if err := os.WriteFile(zipPath, []byte("zip-content-v1"), 0o600); err != nil {
		t.Fatalf("failed to write initial zip artifact: %v", err)
	}

	isoPath := filepath.Join(dir, "output.iso")
	firstHash, err := computeFileSHA256(zipPath)
	if err != nil {
		t.Fatalf("failed to compute initial zip hash: %v", err)
	}

	resource := resourceHyperVIsoImage()
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"source_zip_file_path":      zipPath,
		"destination_iso_file_path": isoPath,
	})
	plannedState := &terraform.InstanceState{
		ID: isoPath,
		Attributes: map[string]string{
			"source_zip_file_path":      zipPath,
			"source_zip_file_path_hash": firstHash,
			"destination_iso_file_path": isoPath,
		},
	}

	if err := os.WriteFile(zipPath, []byte("zip-content-v2"), 0o600); err != nil {
		t.Fatalf("failed to mutate zip artifact: %v", err)
	}

	secondDiff, err := resource.Diff(context.Background(), plannedState, config, nil)
	if err != nil {
		t.Fatalf("second diff failed: %v", err)
	}

	finalState := plannedState.MergeDiff(secondDiff)
	if got := finalState.Attributes["source_zip_file_path_hash"]; got != firstHash {
		t.Fatalf("expected planned zip hash to remain stable across diff re-evaluation, got %q want %q", got, firstHash)
	}

	currentHash, err := computeFileSHA256(zipPath)
	if err != nil {
		t.Fatalf("failed to hash mutated zip artifact: %v", err)
	}
	if currentHash == firstHash {
		t.Fatal("expected the zip artifact contents to change during the test")
	}
}
