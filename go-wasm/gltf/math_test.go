package gltf

import (
	"math"
	"testing"
)

func TestIdentityAndMatrixMultiplication(t *testing.T) {
	identity := Identity()
	wantIdentity := Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	if identity != wantIdentity {
		t.Fatalf("Identity() = %v", identity)
	}

	translation := Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 10, 20, 30, 1}
	scale := Mat4{2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 0, 0, 0, 0, 1}
	combined := multiply(translation, scale)
	point, err := transformPoint(combined, 1, 1, 1)
	if err != nil {
		t.Fatalf("transformPoint returned an error: %v", err)
	}
	if point != [3]float32{12, 23, 34} {
		t.Fatalf("combined transform produced %v", point)
	}
	if got := multiply(identity, combined); got != combined {
		t.Fatalf("left identity changed the matrix: %v", got)
	}
	if got := multiply(combined, identity); got != combined {
		t.Fatalf("right identity changed the matrix: %v", got)
	}
}

func TestNodeMatrixDefaultsAndExplicitMatrix(t *testing.T) {
	got, err := nodeMatrix(nodeDef{})
	if err != nil || got != Identity() {
		t.Fatalf("default node matrix = %v, err = %v", got, err)
	}
	explicit := []float64{2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 0, 5, 6, 7, 1}
	got, err = nodeMatrix(nodeDef{Matrix: explicit})
	if err != nil {
		t.Fatalf("explicit node matrix returned an error: %v", err)
	}
	if got != (Mat4{2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 0, 5, 6, 7, 1}) {
		t.Fatalf("explicit node matrix = %v", got)
	}
}

func TestNodeMatrixComposesTranslationRotationAndScale(t *testing.T) {
	halfRoot := math.Sqrt(0.5)
	got, err := nodeMatrix(nodeDef{
		Translation: []float64{1, 2, 3},
		Rotation:    []float64{0, 0, halfRoot, halfRoot},
		Scale:       []float64{2, 3, 4},
	})
	if err != nil {
		t.Fatalf("nodeMatrix returned an error: %v", err)
	}
	want := Mat4{0, 2, 0, 0, -3, 0, 0, 0, 0, 0, 4, 0, 1, 2, 3, 1}
	assertMatrixClose(t, got, want, 1e-6)
}

func TestNodeMatrixNormalizesQuaternion(t *testing.T) {
	got, err := nodeMatrix(nodeDef{Rotation: []float64{0, 0, 2, 2}})
	if err != nil {
		t.Fatalf("nodeMatrix returned an error: %v", err)
	}
	want, err := nodeMatrix(nodeDef{Rotation: []float64{0, 0, 1, 1}})
	if err != nil {
		t.Fatalf("nodeMatrix returned an error: %v", err)
	}
	assertMatrixClose(t, got, want, 1e-6)
}

func TestNodeMatrixRejectsInvalidTransforms(t *testing.T) {
	tests := []struct {
		name    string
		node    nodeDef
		message string
	}{
		{name: "matrix length", node: nodeDef{Matrix: []float64{1}}, message: "16 values"},
		{name: "matrix and TRS", node: nodeDef{Matrix: make([]float64, 16), Translation: []float64{0, 0, 0}}, message: "both matrix and TRS"},
		{name: "translation length", node: nodeDef{Translation: []float64{1, 2}}, message: "translation must contain 3"},
		{name: "rotation length", node: nodeDef{Rotation: []float64{0, 0, 1}}, message: "rotation must contain 4"},
		{name: "scale length", node: nodeDef{Scale: []float64{1, 1}}, message: "scale must contain 3"},
		{name: "zero quaternion", node: nodeDef{Rotation: []float64{0, 0, 0, 0}}, message: "quaternion is invalid"},
		{name: "matrix range", node: nodeDef{Matrix: []float64{math.MaxFloat64, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}}, message: "supported float range"},
		{name: "translation range", node: nodeDef{Translation: []float64{math.MaxFloat64, 0, 0}}, message: "supported float range"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := nodeMatrix(test.node)
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestTransformPointHandlesHomogeneousCoordinatesAndErrors(t *testing.T) {
	projective := Identity()
	projective[15] = 2
	point, err := transformPoint(projective, 2, 4, 6)
	if err != nil || point != [3]float32{1, 2, 3} {
		t.Fatalf("projective point = %v, err = %v", point, err)
	}

	zeroW := Identity()
	zeroW[15] = 0
	_, err = transformPoint(zeroW, 1, 2, 3)
	assertErrorContains(t, err, "zero homogeneous")

	overflow := Identity()
	overflow[0] = math.MaxFloat32
	_, err = transformPoint(overflow, math.MaxFloat32, 0, 0)
	assertErrorContains(t, err, "non-finite")
}

func TestExpandBounds(t *testing.T) {
	var bounds Bounds
	expandBounds(&bounds, [3]float32{2, -1, 5})
	expandBounds(&bounds, [3]float32{-3, 4, 1})
	if !bounds.Valid || bounds.Min != [3]float32{-3, -1, 1} || bounds.Max != [3]float32{2, 4, 5} {
		t.Fatalf("unexpected bounds: %+v", bounds)
	}
}

func TestFiniteAndFloat32Representability(t *testing.T) {
	if !finite(0) || finite(math.NaN()) || finite(math.Inf(1)) {
		t.Fatal("finite returned an unexpected result")
	}
	if !representableFloat32(math.MaxFloat32) || representableFloat32(math.MaxFloat64) || representableFloat32(math.NaN()) {
		t.Fatal("representableFloat32 returned an unexpected result")
	}
}

func assertMatrixClose(t *testing.T, got, want Mat4, tolerance float64) {
	t.Helper()
	for index := range got {
		if math.Abs(float64(got[index]-want[index])) > tolerance {
			t.Fatalf("matrix[%d] = %v, want %v; got %v", index, got[index], want[index], got)
		}
	}
}
