package gltf

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestParseContainerAcceptsUnknownChunksAfterJSON(t *testing.T) {
	jsonData := []byte(`{"asset":{"version":"2.0"}} `)
	data := makeContainer(
		testChunk{chunkType: jsonChunk, data: jsonData},
		testChunk{chunkType: 0x12345678, data: []byte{1, 2, 3, 4}},
		testChunk{chunkType: binChunk, data: []byte{5, 6, 7, 8}},
	)
	gotJSON, gotBIN, err := parseContainer(data, DefaultLimits())
	if err != nil {
		t.Fatalf("parseContainer returned an error: %v", err)
	}
	if string(gotJSON) != string(jsonData) {
		t.Fatalf("JSON chunk = %q, want %q", gotJSON, jsonData)
	}
	if string(gotBIN) != string([]byte{5, 6, 7, 8}) {
		t.Fatalf("BIN chunk = %v", gotBIN)
	}
}

func TestParseContainerRejectsMalformedData(t *testing.T) {
	valid := makeGLB(`{"asset":{"version":"2.0"}}`, nil)
	badMagic := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badMagic[0:4], 0)
	badVersion := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badVersion[4:8], 1)
	badLength := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(badLength[8:12], uint32(len(badLength)+4))
	firstBIN := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(firstBIN[16:20], binChunk)
	unalignedChunk := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(unalignedChunk[12:16], 3)
	oversizedChunk := append([]byte(nil), valid...)
	binary.LittleEndian.PutUint32(oversizedChunk[12:16], 0xfffffffc)
	truncatedChunkHeader := make([]byte, 16)
	binary.LittleEndian.PutUint32(truncatedChunkHeader[0:4], glbMagic)
	binary.LittleEndian.PutUint32(truncatedChunkHeader[4:8], 2)
	binary.LittleEndian.PutUint32(truncatedChunkHeader[8:12], uint32(len(truncatedChunkHeader)))
	headerOnly := make([]byte, glbHeaderLen)
	binary.LittleEndian.PutUint32(headerOnly[0:4], glbMagic)
	binary.LittleEndian.PutUint32(headerOnly[4:8], 2)
	binary.LittleEndian.PutUint32(headerOnly[8:12], glbHeaderLen)

	cases := []struct {
		name    string
		data    []byte
		message string
	}{
		{name: "truncated header", data: []byte{1, 2, 3}, message: "header is truncated"},
		{name: "bad magic", data: badMagic, message: "invalid GLB magic"},
		{name: "bad version", data: badVersion, message: "only GLB version 2"},
		{name: "declared length mismatch", data: badLength, message: "does not match actual length"},
		{name: "first chunk is not JSON", data: firstBIN, message: "first GLB chunk"},
		{name: "unaligned chunk", data: unalignedChunk, message: "four-byte aligned"},
		{name: "chunk exceeds file", data: oversizedChunk, message: "chunk exceeds"},
		{name: "truncated chunk header", data: truncatedChunkHeader, message: "chunk header is truncated"},
		{name: "no JSON chunk", data: headerOnly, message: "no JSON chunk"},
		{
			name: "duplicate JSON",
			data: makeContainer(
				testChunk{chunkType: jsonChunk, data: []byte("{}  ")},
				testChunk{chunkType: jsonChunk, data: []byte("{}  ")},
			),
			message: "multiple JSON chunks",
		},
		{
			name: "duplicate BIN",
			data: makeContainer(
				testChunk{chunkType: jsonChunk, data: []byte("{}  ")},
				testChunk{chunkType: binChunk, data: []byte{0, 0, 0, 0}},
				testChunk{chunkType: binChunk, data: []byte{0, 0, 0, 0}},
			),
			message: "multiple BIN chunks",
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := parseContainer(test.data, DefaultLimits())
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestParseContainerEnforcesFileAndJSONLimits(t *testing.T) {
	data := makeGLB(`{"asset":{"version":"2.0"}}`, nil)
	t.Run("file", func(t *testing.T) {
		limits := DefaultLimits()
		limits.MaxFileBytes = len(data) - 1
		_, _, err := parseContainer(data, limits)
		assertErrorContains(t, err, "file limit")
	})
	t.Run("JSON", func(t *testing.T) {
		limits := DefaultLimits()
		limits.MaxJSONBytes = 1
		_, _, err := parseContainer(data, limits)
		assertErrorContains(t, err, "JSON exceeds")
	})
}

func TestParseGLBValidatesDocumentAndBufferDeclarations(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		bin     []byte
		message string
	}{
		{name: "invalid JSON", json: `{`, message: "decode GLTF JSON"},
		{name: "missing asset version", json: `{}`, message: "asset.version"},
		{name: "wrong asset version", json: `{"asset":{"version":"1.0"}}`, message: "asset.version"},
		{name: "selected scene absent", json: `{"asset":{"version":"2.0"},"scene":0}`, message: "selected scene index"},
		{name: "multiple buffers", json: `{"asset":{"version":"2.0"},"buffers":[{"byteLength":4},{"byteLength":4}]}`, bin: make([]byte, 4), message: "multiple buffers"},
		{name: "view without buffer", json: `{"asset":{"version":"2.0"},"bufferViews":[{"buffer":0,"byteLength":4}]}`, message: "exactly one embedded buffer"},
		{name: "BIN without buffer", json: `{"asset":{"version":"2.0"}}`, bin: make([]byte, 4), message: "does not declare"},
		{name: "external primary buffer", json: `{"asset":{"version":"2.0"},"buffers":[{"uri":"mesh.bin","byteLength":4}]}`, bin: make([]byte, 4), message: "must use the BIN chunk"},
		{name: "zero buffer length", json: `{"asset":{"version":"2.0"},"buffers":[{"byteLength":0}]}`, bin: []byte{}, message: "buffer length is invalid"},
		{name: "buffer larger than BIN", json: `{"asset":{"version":"2.0"},"buffers":[{"byteLength":8}]}`, bin: make([]byte, 4), message: "buffer length is invalid"},
		{name: "excess BIN padding", json: `{"asset":{"version":"2.0"},"buffers":[{"byteLength":4}]}`, bin: make([]byte, 8), message: "outside the declared buffer"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseGLB(makeGLB(test.json, test.bin))
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestValidateLimitsRejectsEveryNonPositiveLimit(t *testing.T) {
	tests := []struct {
		name string
		set  func(*Limits)
	}{
		{name: "file bytes", set: func(limits *Limits) { limits.MaxFileBytes = 0 }},
		{name: "JSON bytes", set: func(limits *Limits) { limits.MaxJSONBytes = 0 }},
		{name: "texture bytes", set: func(limits *Limits) { limits.MaxTextureBytes = 0 }},
		{name: "total texture bytes", set: func(limits *Limits) { limits.MaxTextureBytesTotal = 0 }},
		{name: "textures", set: func(limits *Limits) { limits.MaxTextures = 0 }},
		{name: "accessors", set: func(limits *Limits) { limits.MaxAccessorCount = 0 }},
		{name: "vertices", set: func(limits *Limits) { limits.MaxVertices = 0 }},
		{name: "indices", set: func(limits *Limits) { limits.MaxIndices = 0 }},
		{name: "primitives", set: func(limits *Limits) { limits.MaxPrimitives = 0 }},
		{name: "nodes", set: func(limits *Limits) { limits.MaxNodes = 0 }},
		{name: "node depth", set: func(limits *Limits) { limits.MaxNodeDepth = 0 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			limits := DefaultLimits()
			test.set(&limits)
			assertErrorContains(t, validateLimits(limits), "must be positive")
		})
	}
}

type testChunk struct {
	chunkType uint32
	data      []byte
}

func makeContainer(chunks ...testChunk) []byte {
	total := glbHeaderLen
	for _, chunk := range chunks {
		if len(chunk.data)%4 != 0 {
			panic("test chunk data must be four-byte aligned")
		}
		total += 8 + len(chunk.data)
	}
	result := make([]byte, total)
	binary.LittleEndian.PutUint32(result[0:4], glbMagic)
	binary.LittleEndian.PutUint32(result[4:8], 2)
	binary.LittleEndian.PutUint32(result[8:12], uint32(total))
	offset := glbHeaderLen
	for _, chunk := range chunks {
		binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(chunk.data)))
		binary.LittleEndian.PutUint32(result[offset+4:offset+8], chunk.chunkType)
		copy(result[offset+8:], chunk.data)
		offset += 8 + len(chunk.data)
	}
	return result
}

func assertErrorContains(t *testing.T, err error, message string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an error containing %q", message)
	}
	if !strings.Contains(err.Error(), message) {
		t.Fatalf("error %q does not contain %q", err, message)
	}
}
