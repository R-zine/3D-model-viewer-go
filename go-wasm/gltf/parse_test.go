package gltf

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestParseGLBAppliesSceneTransformAndMaterial(t *testing.T) {
	bin := triangleBinary()
	jsonData := `{
		"asset":{"version":"2.0"},
        "scene":0,
        "scenes":[{"nodes":[0]}],
        "nodes":[{"mesh":0,"translation":[2,3,4]}],
        "meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1,"material":0}]}],
        "materials":[{"doubleSided":true,"pbrMetallicRoughness":{"baseColorFactor":[0.2,0.4,0.6,0.8]}}],
        "buffers":[{"byteLength":42}],
        "bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":6}],
        "accessors":[
            {"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
            {"bufferView":1,"componentType":5123,"count":3,"type":"SCALAR"}
        ]
    }`
	model, err := ParseGLB(makeGLB(jsonData, bin))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	if len(model.Primitives) != 1 {
		t.Fatalf("got %d primitives, want 1", len(model.Primitives))
	}
	primitive := model.Primitives[0]
	if primitive.Transform[12] != 2 || primitive.Transform[13] != 3 || primitive.Transform[14] != 4 {
		t.Fatalf("node translation was not retained: %v", primitive.Transform)
	}
	if !primitive.Material.DoubleSided || primitive.Material.BaseColorTexture != -1 {
		t.Fatalf("material defaults are incorrect: %+v", primitive.Material)
	}
	if primitive.Material.BaseColorFactor != [4]float32{0.2, 0.4, 0.6, 0.8} {
		t.Fatalf("unexpected base color: %v", primitive.Material.BaseColorFactor)
	}
	if !model.Bounds.Valid || model.Bounds.Min != [3]float32{2, 3, 4} || model.Bounds.Max != [3]float32{3, 4, 4} {
		t.Fatalf("unexpected transformed bounds: %+v", model.Bounds)
	}
	if len(primitive.Normals) != 9 || math.Abs(float64(primitive.Normals[2]-1)) > 1e-6 {
		t.Fatalf("missing normals were not generated: %v", primitive.Normals)
	}
}

func TestParseGLBSupportsNonIndexedPrimitiveAndMissingMaterial(t *testing.T) {
	bin := make([]byte, 24)
	putFloat32(bin[0:4], -1)
	putFloat32(bin[12:16], 1)
	jsonData := `{
		"asset":{"version":"2.0"},
        "meshes":[{"primitives":[{"attributes":{"POSITION":0},"mode":0}]}],
        "materials":[{"pbrMetallicRoughness":{"baseColorTexture":{"index":0}}}],
        "textures":[{}],
        "buffers":[{"byteLength":24}],
        "bufferViews":[{"buffer":0,"byteLength":24}],
        "accessors":[{"bufferView":0,"componentType":5126,"count":2,"type":"VEC3"}]
    }`
	model, err := ParseGLB(makeGLB(jsonData, bin))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	primitive := model.Primitives[0]
	if len(primitive.Indices) != 2 || primitive.Indices[0] != 0 || primitive.Indices[1] != 1 {
		t.Fatalf("non-indexed primitive was not expanded: %v", primitive.Indices)
	}
	if primitive.Material.BaseColorTexture != -1 || len(model.Textures) != 0 {
		t.Fatalf("a missing material incorrectly selected texture zero")
	}
}

func TestParseGLBReadsNormalizedTextureCoordinates(t *testing.T) {
	bin := make([]byte, 44)
	copy(bin, triangleBinary()[:36])
	copy(bin[36:], []byte{0, 0, 255, 0, 0, 255})
	jsonData := `{
		"asset":{"version":"2.0"},
        "meshes":[{"primitives":[{"attributes":{"POSITION":0,"TEXCOORD_0":1}}]}],
        "buffers":[{"byteLength":42}],
        "bufferViews":[{"buffer":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":6}],
        "accessors":[
            {"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
            {"bufferView":1,"componentType":5121,"normalized":true,"count":3,"type":"VEC2"}
        ]
    }`
	model, err := ParseGLB(makeGLB(jsonData, bin))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	if got := model.Primitives[0].UVs; got[2] != 1 || got[5] != 1 {
		t.Fatalf("normalized UV decoding failed: %v", got)
	}
}

func TestGenerateNormalsAvoidsFloat32IntermediateOverflow(t *testing.T) {
	positions := []float32{
		-math.MaxFloat32, -math.MaxFloat32, 0,
		math.MaxFloat32, -math.MaxFloat32, 0,
		0, math.MaxFloat32, 0,
	}
	normals := generateNormals(positions, []uint32{0, 1, 2}, 4)
	for index, value := range normals {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			t.Fatalf("normal %d is non-finite: %v", index, normals)
		}
	}
	if normals[2] != 1 || normals[5] != 1 || normals[8] != 1 {
		t.Fatalf("unexpected extreme-coordinate normals: %v", normals)
	}
}

func TestParseGLBResolvesEmbeddedTextureAndSamplerDefaults(t *testing.T) {
	bin := make([]byte, 60)
	copy(bin, triangleBinary()[:36])
	uvs := []float32{0, 0, 1, 0, 0, 1}
	for index, value := range uvs {
		putFloat32(bin[36+index*4:40+index*4], value)
	}
	jsonData := `{
		"asset":{"version":"2.0"},
		"meshes":[{"primitives":[{"attributes":{"POSITION":0,"TEXCOORD_0":1},"material":0}]}],
		"materials":[{"pbrMetallicRoughness":{"baseColorTexture":{"index":0}}}],
		"textures":[{"source":0}],
		"images":[{"uri":"data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="}],
		"buffers":[{"byteLength":60}],
		"bufferViews":[{"buffer":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":24}],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5126,"count":3,"type":"VEC2"}
		]
	}`
	model, err := ParseGLB(makeGLB(jsonData, bin))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	if len(model.Textures) != 1 || len(model.Textures[0].Data) == 0 {
		t.Fatalf("embedded texture was not resolved: %+v", model.Textures)
	}
	if got := model.Textures[0].Sampler; got != (Sampler{MagFilter: 9729, MinFilter: 9987, WrapS: 10497, WrapT: 10497}) {
		t.Fatalf("unexpected default sampler: %+v", got)
	}
}

func TestParseGLBRejectsUnsupportedTextureCoordinates(t *testing.T) {
	bin := make([]byte, 60)
	copy(bin, triangleBinary()[:36])
	jsonData := `{
		"asset":{"version":"2.0"},
		"meshes":[{"primitives":[{"attributes":{"POSITION":0,"TEXCOORD_0":1},"material":0}]}],
		"materials":[{"pbrMetallicRoughness":{"baseColorTexture":{"index":0,"texCoord":1}}}],
		"textures":[{"source":0}],
		"images":[{"uri":"data:image/png;base64,AA=="}],
		"buffers":[{"byteLength":60}],
		"bufferViews":[{"buffer":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":24}],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5126,"count":3,"type":"VEC2"}
		]
	}`
	if _, err := ParseGLB(makeGLB(jsonData, bin)); err == nil || !strings.Contains(err.Error(), "TEXCOORD_0") {
		t.Fatalf("expected unsupported texture coordinate error, got %v", err)
	}
}

func TestParseGLBRejectsMalformedInputWithoutPanicking(t *testing.T) {
	valid := makeGLB(`{"asset":{"version":"2.0"},"meshes":[]}`, nil)
	cases := map[string][]byte{
		"empty":             nil,
		"truncated header":  {1, 2, 3},
		"length mismatch":   append([]byte(nil), valid[:len(valid)-1]...),
		"negative view":     malformedAccessorGLB(-1),
		"out of range view": malformedAccessorGLB(99),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("parser panicked: %v", recovered)
				}
			}()
			if _, err := ParseGLB(data); err == nil {
				t.Fatal("ParseGLB accepted malformed input")
			}
		})
	}
}

func TestParseGLBRejectsNodeCycles(t *testing.T) {
	jsonData := `{"asset":{"version":"2.0"},"scene":0,"scenes":[{"nodes":[0]}],"nodes":[{"children":[0]}],"meshes":[]}`
	if _, err := ParseGLB(makeGLB(jsonData, nil)); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

func TestParseGLBRejectsUnsupportedRequiredFeatures(t *testing.T) {
	cases := map[string]string{
		"required extension": `{"asset":{"version":"2.0"},"extensionsRequired":["KHR_draco_mesh_compression"]}`,
		"matrix and TRS":     `{"asset":{"version":"2.0"},"nodes":[{"matrix":[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],"translation":[1,2,3]}]}`,
		"multiple parents":   `{"asset":{"version":"2.0"},"nodes":[{"children":[2]},{"children":[2]},{}]}`,
	}
	for name, jsonData := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseGLB(makeGLB(jsonData, nil)); err == nil {
				t.Fatal("ParseGLB accepted an unsupported or invalid feature")
			}
		})
	}
}

func TestParseGLBEnforcesModelWideGeometryLimits(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"meshes":[{"primitives":[
			{"attributes":{"POSITION":0}},
			{"attributes":{"POSITION":0}}
		]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	limits := DefaultLimits()
	limits.MaxVertices = 5
	if _, err := ParseGLBWithLimits(makeGLB(jsonData, triangleBinary()[:36]), limits); err == nil || !strings.Contains(err.Error(), "total vertex limit") {
		t.Fatalf("expected total vertex limit error, got %v", err)
	}
}

func TestParseGLBEnforcesInstancedGeometryLimits(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"scene":0,
		"scenes":[{"nodes":[0,1]}],
		"nodes":[{"mesh":0},{"mesh":0}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	limits := DefaultLimits()
	limits.MaxVertices = 5
	if _, err := ParseGLBWithLimits(makeGLB(jsonData, triangleBinary()[:36]), limits); err == nil || !strings.Contains(err.Error(), "selected scene") {
		t.Fatalf("expected instanced vertex limit error, got %v", err)
	}
}

func TestParseGLBRejectsMissingTextureIndex(t *testing.T) {
	bin := make([]byte, 60)
	copy(bin, triangleBinary()[:36])
	jsonData := `{
		"asset":{"version":"2.0"},
		"meshes":[{"primitives":[{"attributes":{"POSITION":0,"TEXCOORD_0":1},"material":0}]}],
		"materials":[{"pbrMetallicRoughness":{"baseColorTexture":{}}}],
		"buffers":[{"byteLength":60}],
		"bufferViews":[{"buffer":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":24}],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5126,"count":3,"type":"VEC2"}
		]
	}`
	if _, err := ParseGLB(makeGLB(jsonData, bin)); err == nil || !strings.Contains(err.Error(), "index is required") {
		t.Fatalf("expected missing texture index error, got %v", err)
	}
}

func TestParseGLBDoesNotRenderMeshesOutsideAnExplicitEmptyScene(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"scene":0,
		"scenes":[{"nodes":[]}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	if _, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36])); err == nil || !strings.Contains(err.Error(), "no renderable primitives") {
		t.Fatalf("expected empty selected scene error, got %v", err)
	}
}

func FuzzParseGLB(f *testing.F) {
	f.Add([]byte{})
	f.Add(makeGLB(`{"asset":{"version":"2.0"},"meshes":[]}`, nil))
	f.Add(makeGLB(`{"asset":{"version":"2.0"},"accessors":[{"bufferView":-1,"componentType":5126,"count":1,"type":"VEC3"}]}`, nil))
	f.Add(makeGLB(`{"asset":{"version":"2.0"},"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],"buffers":[{"byteLength":36}],"bufferViews":[{"buffer":0,"byteLength":36}],"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]}`, triangleBinary()[:36]))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("ParseGLB panicked for %d bytes: %v", len(data), recovered)
			}
		}()
		_, _ = ParseGLB(data)
	})
}

func FuzzParseGLBJSON(f *testing.F) {
	f.Add(`{"asset":{"version":"2.0"}}`, []byte{})
	f.Add(`{"asset":{"version":"2.0"},"nodes":[{"children":[0]}]}`, []byte{})
	f.Add(`{"asset":{"version":"2.0"},"buffers":[{"byteLength":4}]}`, []byte{0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, jsonData string, bin []byte) {
		if len(jsonData) > 1<<20 || len(bin) > 1<<20 {
			t.Skip()
		}
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("ParseGLB panicked for wrapped JSON/BIN data: %v", recovered)
			}
		}()
		_, _ = ParseGLB(makeGLB(jsonData, bin))
	})
}

func malformedAccessorGLB(bufferView int) []byte {
	jsonData := `{
		"asset":{"version":"2.0"},
        "meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
        "buffers":[{"byteLength":12}],
        "bufferViews":[{"buffer":0,"byteLength":12}],
        "accessors":[{"bufferView":BUFFER_VIEW,"componentType":5126,"count":1,"type":"VEC3"}]
    }`
	jsonData = strings.Replace(jsonData, "BUFFER_VIEW", stringInt(bufferView), 1)
	return makeGLB(jsonData, make([]byte, 12))
}

func stringInt(value int) string {
	if value == -1 {
		return "-1"
	}
	if value == 99 {
		return "99"
	}
	panic("test helper received an unexpected integer")
}

func triangleBinary() []byte {
	data := make([]byte, 44)
	positions := []float32{0, 0, 0, 1, 0, 0, 0, 1, 0}
	for index, value := range positions {
		putFloat32(data[index*4:index*4+4], value)
	}
	binary.LittleEndian.PutUint16(data[36:38], 0)
	binary.LittleEndian.PutUint16(data[38:40], 1)
	binary.LittleEndian.PutUint16(data[40:42], 2)
	return data
}

func putFloat32(destination []byte, value float32) {
	binary.LittleEndian.PutUint32(destination, math.Float32bits(value))
}

func makeGLB(jsonData string, bin []byte) []byte {
	jsonChunkData := append([]byte(nil), []byte(jsonData)...)
	for len(jsonChunkData)%4 != 0 {
		jsonChunkData = append(jsonChunkData, ' ')
	}
	binChunkData := append([]byte(nil), bin...)
	for len(binChunkData)%4 != 0 {
		binChunkData = append(binChunkData, 0)
	}
	totalLength := 12 + 8 + len(jsonChunkData)
	if bin != nil {
		totalLength += 8 + len(binChunkData)
	}
	result := make([]byte, totalLength)
	binary.LittleEndian.PutUint32(result[0:4], glbMagic)
	binary.LittleEndian.PutUint32(result[4:8], 2)
	binary.LittleEndian.PutUint32(result[8:12], uint32(totalLength))
	offset := 12
	binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(jsonChunkData)))
	binary.LittleEndian.PutUint32(result[offset+4:offset+8], jsonChunk)
	copy(result[offset+8:], jsonChunkData)
	offset += 8 + len(jsonChunkData)
	if bin != nil {
		binary.LittleEndian.PutUint32(result[offset:offset+4], uint32(len(binChunkData)))
		binary.LittleEndian.PutUint32(result[offset+4:offset+8], binChunk)
		copy(result[offset+8:], binChunkData)
	}
	return result
}
