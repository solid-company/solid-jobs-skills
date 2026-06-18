package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	archive := []byte("pretend binary archive")
	sum := sha256.Sum256(archive)
	good := hex.EncodeToString(sum[:])
	sums := []byte(fmt.Sprintf("deadbeef  sjctl_other.tar.gz\n%s  sjctl_linux_amd64.tar.gz\n", good))

	if err := verifyChecksum(archive, sums, "sjctl_linux_amd64.tar.gz"); err != nil {
		t.Fatalf("matching checksum should verify: %v", err)
	}

	if err := verifyChecksum([]byte("tampered"), sums, "sjctl_linux_amd64.tar.gz"); err == nil {
		t.Fatal("tampered archive must fail verification")
	}

	if err := verifyChecksum(archive, sums, "sjctl_missing.tar.gz"); err == nil {
		t.Fatal("absent checksum entry must fail verification")
	}
}

func TestExtractFromTarGz(t *testing.T) {
	want := []byte("\x7fELF the sjctl binary")
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range []struct {
		name string
		data []byte
	}{
		{"README.md", []byte("docs")},
		{"sjctl", want},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Size: int64(len(e.data)), Mode: 0o755}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.data); err != nil {
			t.Fatal(err)
		}
	}
	tw.Close()
	gz.Close()

	got, err := extractFromTarGz(buf.Bytes(), "sjctl")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}

	if _, err := extractFromTarGz(buf.Bytes(), "absent"); err == nil {
		t.Fatal("missing entry should error")
	}
}

func TestExtractFromZip(t *testing.T) {
	want := []byte("MZ the sjctl.exe binary")
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("sjctl.exe")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(want); err != nil {
		t.Fatal(err)
	}
	zw.Close()

	got, err := extractFromZip(buf.Bytes(), "sjctl.exe")
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
