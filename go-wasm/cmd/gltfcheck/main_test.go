package main

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"viewer/gltf"
)

func TestRunValidatesAndSummarizesGLB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "point.glb")
	if err := os.WriteFile(path, pointGLB(), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{path}, &stdout, &stderr); code != 0 {
		t.Fatalf("run returned %d; stderr: %s", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, "1 primitives, 1 vertices, 1 indices, 0 base-color textures") {
		t.Fatalf("unexpected summary: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRunReportsUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"one.glb", "two.glb"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("run(%v) returned %d", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: gltfcheck") || stdout.Len() != 0 {
			t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
		}
	}
}

func TestRunReportsFilesystemAndParserErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{filepath.Join(t.TempDir(), "missing.glb")}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "invalid GLB:") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
	t.Run("invalid GLB", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.glb")
		if err := os.WriteFile(path, []byte("invalid"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		var stdout, stderr bytes.Buffer
		code := run([]string{path}, &stdout, &stderr)
		if code != 1 || !strings.Contains(stderr.String(), "header is truncated") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
	t.Run("file limit", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "large.glb")
		if err := os.WriteFile(path, []byte{1, 2, 3, 4, 5}, 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		limits := gltf.DefaultLimits()
		limits.MaxFileBytes = 4
		var stdout, stderr bytes.Buffer
		code := runWithLimits([]string{path}, &stdout, &stderr, limits)
		if code != 1 || !strings.Contains(stderr.String(), "4-byte parser limit") || stdout.Len() != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
		}
	})
}

func TestInvalidFormatsError(t *testing.T) {
	var stderr bytes.Buffer
	if code := invalid(&stderr, os.ErrInvalid); code != 1 {
		t.Fatalf("invalid returned %d", code)
	}
	if !strings.Contains(stderr.String(), "invalid GLB:") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func pointGLB() []byte {
	jsonData := []byte(`{"asset":{"version":"2.0"},"meshes":[{"primitives":[{"attributes":{"POSITION":0},"mode":0}]}],"buffers":[{"byteLength":12}],"bufferViews":[{"buffer":0,"byteLength":12}],"accessors":[{"bufferView":0,"componentType":5126,"count":1,"type":"VEC3"}]}`)
	for len(jsonData)%4 != 0 {
		jsonData = append(jsonData, ' ')
	}
	bin := make([]byte, 12)
	binary.LittleEndian.PutUint32(bin[0:4], math.Float32bits(1))
	total := 12 + 8 + len(jsonData) + 8 + len(bin)
	result := make([]byte, total)
	binary.LittleEndian.PutUint32(result[0:4], 0x46546c67)
	binary.LittleEndian.PutUint32(result[4:8], 2)
	binary.LittleEndian.PutUint32(result[8:12], uint32(total))
	binary.LittleEndian.PutUint32(result[12:16], uint32(len(jsonData)))
	binary.LittleEndian.PutUint32(result[16:20], 0x4e4f534a)
	copy(result[20:], jsonData)
	offset := 20 + len(jsonData)
	binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(bin)))
	binary.LittleEndian.PutUint32(result[offset+4:offset+8], 0x004e4942)
	copy(result[offset+8:], bin)
	return result
}
