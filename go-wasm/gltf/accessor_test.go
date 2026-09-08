package gltf

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestReadFloatComponent(t *testing.T) {
	uint16Bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(uint16Bytes, math.MaxUint16)
	int16Bytes := make([]byte, 2)
	binary.LittleEndian.PutUint16(int16Bytes, 0x8000)
	uint32Bytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(uint32Bytes, math.MaxUint32)
	floatBytes := make([]byte, 4)
	putFloat32(floatBytes, 1.25)

	tests := []struct {
		name          string
		data          []byte
		componentType int
		normalized    bool
		want          float32
	}{
		{name: "signed byte", data: []byte{0xfe}, componentType: 5120, want: -2},
		{name: "normalized signed byte clamps minimum", data: []byte{0x80}, componentType: 5120, normalized: true, want: -1},
		{name: "unsigned byte", data: []byte{200}, componentType: 5121, want: 200},
		{name: "normalized unsigned byte", data: []byte{255}, componentType: 5121, normalized: true, want: 1},
		{name: "normalized signed short clamps minimum", data: int16Bytes, componentType: 5122, normalized: true, want: -1},
		{name: "normalized unsigned short", data: uint16Bytes, componentType: 5123, normalized: true, want: 1},
		{name: "unsigned int", data: uint32Bytes, componentType: 5125, want: float32(math.MaxUint32)},
		{name: "float", data: floatBytes, componentType: 5126, want: 1.25},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := readFloatComponent(test.data, test.componentType, test.normalized)
			if err != nil {
				t.Fatalf("readFloatComponent returned an error: %v", err)
			}
			if got != test.want {
				t.Fatalf("readFloatComponent = %v, want %v", got, test.want)
			}
		})
	}
	if _, err := readFloatComponent([]byte{0}, 9999, false); err == nil {
		t.Fatal("unsupported component type was accepted")
	}
}

func TestAccessorTypeAndComponentMetadata(t *testing.T) {
	typeTests := map[string]int{
		"SCALAR": 1,
		"VEC2":   2,
		"VEC3":   3,
		"VEC4":   4,
		"MAT2":   4,
		"MAT3":   9,
		"MAT4":   16,
	}
	for accessorType, want := range typeTests {
		got, ok := typeComponentCount(accessorType)
		if !ok || got != want {
			t.Errorf("typeComponentCount(%q) = %d, %v; want %d, true", accessorType, got, ok, want)
		}
	}
	if _, ok := typeComponentCount("INVALID"); ok {
		t.Error("invalid accessor type was accepted")
	}

	for _, componentType := range []int{5120, 5121} {
		if size, ok := componentSize(componentType); !ok || size != 1 {
			t.Errorf("componentSize(%d) = %d, %v", componentType, size, ok)
		}
	}
	for _, componentType := range []int{5122, 5123} {
		if size, ok := componentSize(componentType); !ok || size != 2 {
			t.Errorf("componentSize(%d) = %d, %v", componentType, size, ok)
		}
	}
	for _, componentType := range []int{5125, 5126} {
		if size, ok := componentSize(componentType); !ok || size != 4 {
			t.Errorf("componentSize(%d) = %d, %v", componentType, size, ok)
		}
	}
	if _, ok := componentSize(0); ok {
		t.Error("invalid component type was accepted")
	}
}

func TestCheckedIntegerArithmetic(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if got, ok := checkedAdd(2, 3); !ok || got != 5 {
		t.Fatalf("checkedAdd(2, 3) = %d, %v", got, ok)
	}
	for _, values := range [][2]int{{-1, 1}, {1, -1}, {maxInt, 1}} {
		if _, ok := checkedAdd(values[0], values[1]); ok {
			t.Errorf("checkedAdd(%d, %d) did not reject overflow/negative input", values[0], values[1])
		}
	}
	if got, ok := checkedMultiply(6, 7); !ok || got != 42 {
		t.Fatalf("checkedMultiply(6, 7) = %d, %v", got, ok)
	}
	for _, values := range [][2]int{{-1, 1}, {1, -1}, {maxInt, 2}} {
		if _, ok := checkedMultiply(values[0], values[1]); ok {
			t.Errorf("checkedMultiply(%d, %d) did not reject overflow/negative input", values[0], values[1])
		}
	}
}

func TestAccessorReaderReadsInterleavedPositions(t *testing.T) {
	bin := make([]byte, 36)
	putFloat32(bin[4:8], 1)
	putFloat32(bin[8:12], 2)
	putFloat32(bin[12:16], 3)
	putFloat32(bin[20:24], 4)
	putFloat32(bin[24:28], 5)
	putFloat32(bin[28:32], 6)
	reader := testAccessorReader(
		accessorDef{BufferView: intPointer(0), ComponentType: 5126, Count: 2, Type: "VEC3"},
		bufferViewDef{Buffer: 0, ByteOffset: 4, ByteLength: 32, ByteStride: 16},
		bin,
	)
	got, count, err := reader.positions(0)
	if err != nil {
		t.Fatalf("positions returned an error: %v", err)
	}
	if count != 2 || !equalFloat32s(got, []float32{1, 2, 3, 4, 5, 6}) {
		t.Fatalf("positions = %v (count %d)", got, count)
	}
}

func TestAccessorReaderReadsNormalizedAttributes(t *testing.T) {
	t.Run("normals", func(t *testing.T) {
		reader := testAccessorReader(
			accessorDef{BufferView: intPointer(0), ComponentType: 5120, Count: 1, Type: "VEC3", Normalized: true},
			bufferViewDef{Buffer: 0, ByteLength: 3},
			[]byte{0, 0, 127},
		)
		got, err := reader.normals(0, 1)
		if err != nil || !equalFloat32s(got, []float32{0, 0, 1}) {
			t.Fatalf("normals = %v, err = %v", got, err)
		}
	})
	t.Run("texture coordinates", func(t *testing.T) {
		bin := make([]byte, 4)
		binary.LittleEndian.PutUint16(bin[2:4], math.MaxUint16)
		reader := testAccessorReader(
			accessorDef{BufferView: intPointer(0), ComponentType: 5123, Count: 1, Type: "VEC2", Normalized: true},
			bufferViewDef{Buffer: 0, ByteLength: 4},
			bin,
		)
		got, err := reader.textureCoordinates(0, 1)
		if err != nil || !equalFloat32s(got, []float32{0, 1}) {
			t.Fatalf("texture coordinates = %v, err = %v", got, err)
		}
	})
}

func TestAccessorReaderReadsAllUnsignedIndexTypes(t *testing.T) {
	tests := []struct {
		name          string
		componentType int
		data          []byte
	}{
		{name: "unsigned byte", componentType: 5121, data: []byte{0, 1, 2}},
		{name: "unsigned short", componentType: 5123, data: []byte{0, 0, 1, 0, 2, 0}},
		{name: "unsigned int", componentType: 5125, data: []byte{0, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := testAccessorReader(
				accessorDef{BufferView: intPointer(0), ComponentType: test.componentType, Count: 3, Type: "SCALAR"},
				bufferViewDef{Buffer: 0, ByteLength: len(test.data)},
				test.data,
			)
			got, err := reader.indices(0, 3)
			if err != nil || len(got) != 3 || got[0] != 0 || got[1] != 1 || got[2] != 2 {
				t.Fatalf("indices = %v, err = %v", got, err)
			}
		})
	}
}

func TestAccessorReaderRejectsInvalidPositionLayouts(t *testing.T) {
	baseAccessor := accessorDef{BufferView: intPointer(0), ComponentType: 5126, Count: 1, Type: "VEC3"}
	baseView := bufferViewDef{Buffer: 0, ByteLength: 12}
	tests := []struct {
		name    string
		mutate  func(*accessorDef, *bufferViewDef, *accessorReader)
		message string
	}{
		{name: "wrong type", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.Type = "VEC2" }, message: "must have type VEC3"},
		{name: "zero count", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.Count = 0 }, message: "invalid count"},
		{name: "accessor count limit", mutate: func(a *accessorDef, _ *bufferViewDef, r *accessorReader) { a.Count = 2; r.limits.MaxAccessorCount = 1 }, message: "invalid count"},
		{name: "vertex limit", mutate: func(a *accessorDef, _ *bufferViewDef, r *accessorReader) { a.Count = 2; r.limits.MaxVertices = 1 }, message: "vertex limit"},
		{name: "normalized positions", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.Normalized = true }, message: "non-normalized FLOAT"},
		{name: "integer positions", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.ComponentType = 5123 }, message: "non-normalized FLOAT"},
		{name: "negative accessor offset", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.ByteOffset = -1 }, message: "negative byte offset"},
		{name: "sparse", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.Sparse = &accessorSparse{} }, message: "sparse accessor"},
		{name: "missing view", mutate: func(a *accessorDef, _ *bufferViewDef, _ *accessorReader) { a.BufferView = nil }, message: "no bufferView"},
		{name: "secondary buffer", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.Buffer = 1 }, message: "secondary buffers"},
		{name: "negative view offset", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteOffset = -1 }, message: "negative offset"},
		{name: "view beyond BIN", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteLength = 17 }, message: "exceeds the BIN"},
		{name: "stride too small", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteStride = 8 }, message: "invalid byteStride"},
		{name: "stride too large", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteStride = 256 }, message: "invalid byteStride"},
		{name: "stride misaligned", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteStride = 14 }, message: "invalid byteStride"},
		{name: "accessor misaligned", mutate: func(a *accessorDef, v *bufferViewDef, _ *accessorReader) { a.ByteOffset = 1; v.ByteLength = 13 }, message: "not aligned"},
		{name: "view misaligned", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteOffset = 1; v.ByteLength = 12 }, message: "not aligned"},
		{name: "accessor exceeds view", mutate: func(_ *accessorDef, v *bufferViewDef, _ *accessorReader) { v.ByteLength = 11 }, message: "exceeds its bufferView"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			accessor, view := baseAccessor, baseView
			bin := make([]byte, 16)
			reader := testAccessorReader(accessor, view, bin)
			test.mutate(&reader.document.Accessors[0], &reader.document.BufferViews[0], &reader)
			_, _, err := reader.positions(0)
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestAccessorReaderRejectsInvalidAttributeAndIndexEncodings(t *testing.T) {
	t.Run("normal count mismatch", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5126, Count: 1, Type: "VEC3"}, bufferViewDef{ByteLength: 12}, make([]byte, 12))
		_, err := reader.normals(0, 2)
		assertErrorContains(t, err, "does not match POSITION")
	})
	t.Run("normal encoding", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5120, Count: 1, Type: "VEC3"}, bufferViewDef{ByteLength: 3}, make([]byte, 3))
		_, err := reader.normals(0, 1)
		assertErrorContains(t, err, "unsupported component encoding")
	})
	t.Run("UV count mismatch", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5126, Count: 1, Type: "VEC2"}, bufferViewDef{ByteLength: 8}, make([]byte, 8))
		_, err := reader.textureCoordinates(0, 2)
		assertErrorContains(t, err, "does not match POSITION")
	})
	t.Run("UV encoding", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5121, Count: 1, Type: "VEC2"}, bufferViewDef{ByteLength: 2}, make([]byte, 2))
		_, err := reader.textureCoordinates(0, 1)
		assertErrorContains(t, err, "unsupported component encoding")
	})
	t.Run("signed indices", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5122, Count: 1, Type: "SCALAR"}, bufferViewDef{ByteLength: 2}, make([]byte, 2))
		_, err := reader.indices(0, 1)
		assertErrorContains(t, err, "unsigned integer")
	})
	t.Run("normalized indices", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5121, Count: 1, Type: "SCALAR", Normalized: true}, bufferViewDef{ByteLength: 1}, []byte{0})
		_, err := reader.indices(0, 1)
		assertErrorContains(t, err, "unsigned integer")
	})
	t.Run("index count limit", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5121, Count: 2, Type: "SCALAR"}, bufferViewDef{ByteLength: 2}, []byte{0, 0})
		reader.limits.MaxIndices = 1
		_, err := reader.indices(0, 1)
		assertErrorContains(t, err, "index limit")
	})
	t.Run("index outside vertex range", func(t *testing.T) {
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5121, Count: 1, Type: "SCALAR"}, bufferViewDef{ByteLength: 1}, []byte{3})
		_, err := reader.indices(0, 3)
		assertErrorContains(t, err, "only 3 vertices")
	})
	t.Run("non-finite float", func(t *testing.T) {
		bin := make([]byte, 12)
		binary.LittleEndian.PutUint32(bin[0:4], math.Float32bits(float32(math.NaN())))
		reader := testAccessorReader(accessorDef{BufferView: intPointer(0), ComponentType: 5126, Count: 1, Type: "VEC3"}, bufferViewDef{ByteLength: 12}, bin)
		_, _, err := reader.positions(0)
		assertErrorContains(t, err, "non-finite")
	})
}

func testAccessorReader(accessor accessorDef, view bufferViewDef, bin []byte) accessorReader {
	document := &document{Accessors: []accessorDef{accessor}, BufferViews: []bufferViewDef{view}}
	return accessorReader{document: document, bin: bin, limits: DefaultLimits()}
}

func intPointer(value int) *int {
	return &value
}

func equalFloat32s(left, right []float32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
