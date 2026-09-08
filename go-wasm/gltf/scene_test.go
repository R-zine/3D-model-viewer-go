package gltf

import (
	"strings"
	"testing"
)

func TestValidateTopology(t *testing.T) {
	tests := []struct {
		name       string
		mode       uint32
		count      int
		wantErr    bool
		errorMatch string
	}{
		{name: "points", mode: 0, count: 1},
		{name: "lines", mode: 1, count: 4},
		{name: "lines odd", mode: 1, count: 3, wantErr: true, errorMatch: "even number"},
		{name: "line loop", mode: 2, count: 2},
		{name: "line strip", mode: 3, count: 2},
		{name: "line strip too short", mode: 3, count: 1, wantErr: true, errorMatch: "at least two"},
		{name: "triangles", mode: 4, count: 6},
		{name: "triangles incomplete", mode: 4, count: 4, wantErr: true, errorMatch: "divisible by three"},
		{name: "triangle strip", mode: 5, count: 3},
		{name: "triangle fan", mode: 6, count: 3},
		{name: "triangle fan too short", mode: 6, count: 2, wantErr: true, errorMatch: "at least three"},
		{name: "unknown", mode: 7, count: 3, wantErr: true, errorMatch: "unsupported primitive mode"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTopology(test.mode, test.count)
			if test.wantErr {
				assertErrorContains(t, err, test.errorMatch)
			} else if err != nil {
				t.Fatalf("validateTopology returned an error: %v", err)
			}
		})
	}
}

func TestGenerateNormalsForTriangleModes(t *testing.T) {
	positions := []float32{
		0, 0, 0,
		1, 0, 0,
		1, 1, 0,
		0, 1, 0,
	}
	tests := []struct {
		name    string
		mode    uint32
		indices []uint32
	}{
		{name: "triangles", mode: 4, indices: []uint32{0, 1, 2, 0, 2, 3}},
		{name: "triangle strip", mode: 5, indices: []uint32{0, 1, 3, 2}},
		{name: "triangle fan", mode: 6, indices: []uint32{0, 1, 2, 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normals := generateNormals(positions, test.indices, test.mode)
			for vertex := 0; vertex < 4; vertex++ {
				got := normals[vertex*3 : vertex*3+3]
				if !equalFloat32s(got, []float32{0, 0, 1}) {
					t.Fatalf("vertex %d normal = %v; all normals = %v", vertex, got, normals)
				}
			}
		})
	}
}

func TestGenerateNormalsUsesStableFallbackForNonTrianglesAndDegenerates(t *testing.T) {
	tests := []struct {
		name      string
		positions []float32
		indices   []uint32
		mode      uint32
	}{
		{name: "points", positions: []float32{1, 2, 3}, indices: []uint32{0}, mode: 0},
		{name: "lines", positions: []float32{0, 0, 0, 1, 0, 0}, indices: []uint32{0, 1}, mode: 1},
		{name: "degenerate triangle", positions: []float32{0, 0, 0, 1, 0, 0, 2, 0, 0}, indices: []uint32{0, 1, 2}, mode: 4},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normals := generateNormals(test.positions, test.indices, test.mode)
			for vertex := 0; vertex < len(test.positions)/3; vertex++ {
				if got := normals[vertex*3 : vertex*3+3]; !equalFloat32s(got, []float32{0, 0, 1}) {
					t.Fatalf("fallback normal = %v", got)
				}
			}
		})
	}
}

func TestParseGLBComposesNodeHierarchyTransforms(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"scene":0,
		"scenes":[{"nodes":[0]}],
		"nodes":[{"translation":[1,0,0],"children":[1]},{"translation":[0,2,0],"mesh":0}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	model, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36]))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	primitive := model.Primitives[0]
	if primitive.Transform[12] != 1 || primitive.Transform[13] != 2 || primitive.Transform[14] != 0 {
		t.Fatalf("composed transform = %v", primitive.Transform)
	}
	if model.Bounds.Min != [3]float32{1, 2, 0} || model.Bounds.Max != [3]float32{2, 3, 0} {
		t.Fatalf("composed bounds = %+v", model.Bounds)
	}
}

func TestParseGLBInstantiatesOnlyTheSelectedScene(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"scene":1,
		"scenes":[{"nodes":[0]},{"nodes":[1]}],
		"nodes":[{"translation":[1,0,0],"mesh":0},{"translation":[5,0,0],"mesh":0}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	model, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36]))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	if len(model.Primitives) != 1 || model.Primitives[0].Transform[12] != 5 {
		t.Fatalf("selected scene primitives = %+v", model.Primitives)
	}
}

func TestParseGLBInfersRootNodesWhenScenesAreAbsent(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"nodes":[{"children":[1]},{"mesh":0},{"translation":[3,0,0],"mesh":0}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
		"buffers":[{"byteLength":36}],
		"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	model, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36]))
	if err != nil {
		t.Fatalf("ParseGLB returned an error: %v", err)
	}
	if len(model.Primitives) != 2 {
		t.Fatalf("got %d primitives, want 2", len(model.Primitives))
	}
	if model.Bounds.Min != [3]float32{0, 0, 0} || model.Bounds.Max != [3]float32{4, 1, 0} {
		t.Fatalf("inferred-scene bounds = %+v", model.Bounds)
	}
}

func TestParseGLBRejectsInvalidSceneGraphs(t *testing.T) {
	cases := []struct {
		name    string
		json    string
		message string
	}{
		{name: "invalid child", json: `{"asset":{"version":"2.0"},"nodes":[{"children":[1]}]}`, message: "invalid child index"},
		{name: "invalid mesh", json: `{"asset":{"version":"2.0"},"nodes":[{"mesh":0}]}`, message: "invalid mesh index"},
		{name: "unsupported skin", json: `{"asset":{"version":"2.0"},"nodes":[{"skin":0}]}`, message: "unsupported skin"},
		{name: "duplicate scene root", json: `{"asset":{"version":"2.0"},"scenes":[{"nodes":[0,0]}],"nodes":[{}]}`, message: "more than once"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseGLB(makeGLB(test.json, nil))
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestParseGLBRejectsNodeDepthAndRepeatedSceneReferences(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		jsonData := `{"asset":{"version":"2.0"},"nodes":[{"children":[1]},{"children":[2]},{}]}`
		limits := DefaultLimits()
		limits.MaxNodeDepth = 2
		_, err := ParseGLBWithLimits(makeGLB(jsonData, nil), limits)
		assertErrorContains(t, err, "depth limit")
	})
	t.Run("root and descendant", func(t *testing.T) {
		jsonData := `{
			"asset":{"version":"2.0"},"scene":0,"scenes":[{"nodes":[0,1]}],
			"nodes":[{"children":[1]},{"mesh":0}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}],
			"buffers":[{"byteLength":36}],"bufferViews":[{"buffer":0,"byteLength":36}],
			"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
		}`
		_, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36]))
		assertErrorContains(t, err, "referenced more than once")
	})
}

func TestParseGLBRejectsMorphTargets(t *testing.T) {
	jsonData := `{
		"asset":{"version":"2.0"},
		"meshes":[{"primitives":[{"attributes":{"POSITION":0},"targets":[{"POSITION":0}]}]}],
		"buffers":[{"byteLength":36}],"bufferViews":[{"buffer":0,"byteLength":36}],
		"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}]
	}`
	_, err := ParseGLB(makeGLB(jsonData, triangleBinary()[:36]))
	if err == nil || !strings.Contains(err.Error(), "morph targets") {
		t.Fatalf("expected morph target error, got %v", err)
	}
}
