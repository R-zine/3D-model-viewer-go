package renderer

import (
	"math"
	"testing"
	"viewer/gltf"
)

func TestMultiplyMatricesPreservesIdentity(t *testing.T) {
	matrix := gltf.Mat4{2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 0, 5, 6, 7, 1}
	if got := multiplyMatrices(gltf.Identity(), matrix); got != matrix {
		t.Fatalf("identity multiplication changed matrix: %v", got)
	}
}

func TestMultiplyMatricesUsesColumnMajorCompositionOrder(t *testing.T) {
	translation := gltf.Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 10, 20, 30, 1}
	scale := gltf.Mat4{2, 0, 0, 0, 0, 3, 0, 0, 0, 0, 4, 0, 0, 0, 0, 1}
	got := transformTestPoint(multiplyMatrices(translation, scale), [3]float32{1, 1, 1})
	if got != [3]float32{12, 23, 34} {
		t.Fatalf("composed point = %v", got)
	}
	reversed := transformTestPoint(multiplyMatrices(scale, translation), [3]float32{1, 1, 1})
	if reversed != [3]float32{22, 63, 124} {
		t.Fatalf("reverse-composed point = %v", reversed)
	}
}

func TestPerspectiveProjection(t *testing.T) {
	got := perspective(90, 2, 1, 11)
	want := gltf.Mat4{0.5, 0, 0, 0, 0, 1, 0, 0, 0, 0, -1.2, -1, 0, 0, -2.2, 0}
	assertRendererMatrixClose(t, got, want, 1e-6)
}

func TestRotationMatrixRotatesAroundBothAxes(t *testing.T) {
	aroundX := transformTestPoint(rotationMatrix(math.Pi/2, 0), [3]float32{0, 1, 0})
	assertPointClose(t, aroundX, [3]float32{0, 0, 1}, 1e-6)
	aroundY := transformTestPoint(rotationMatrix(0, math.Pi/2), [3]float32{0, 0, 1})
	assertPointClose(t, aroundY, [3]float32{1, 0, 0}, 1e-6)
}

func TestFitToViewCentersAndScalesBounds(t *testing.T) {
	fit := fitToView(gltf.Bounds{Min: [3]float32{2, 4, 6}, Max: [3]float32{6, 6, 8}, Valid: true})
	if fit[0] != 0.5 || fit[5] != 0.5 || fit[10] != 0.5 {
		t.Fatalf("unexpected fit scale: %v", fit)
	}
	if fit[12] != -2 || fit[13] != -2.5 || fit[14] != -3.5 {
		t.Fatalf("unexpected fit translation: %v", fit)
	}
}

func TestFitToViewHandlesInvalidAndDegenerateBounds(t *testing.T) {
	if got := fitToView(gltf.Bounds{}); got != gltf.Identity() {
		t.Fatalf("invalid bounds fit = %v", got)
	}
	got := fitToView(gltf.Bounds{Min: [3]float32{2, 3, 4}, Max: [3]float32{2, 3, 4}, Valid: true})
	want := gltf.Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, -2, -3, -4, 1}
	if got != want {
		t.Fatalf("degenerate bounds fit = %v", got)
	}
}

func TestNormalMatrixUsesInverseTranspose(t *testing.T) {
	matrix := gltf.Mat4{2, 0, 0, 0, 0, 4, 0, 0, 0, 0, 8, 0, 0, 0, 0, 1}
	got := normalMatrix(matrix)
	want := [9]float32{0.5, 0, 0, 0, 0.25, 0, 0, 0, 0.125}
	for index := range got {
		if math.Abs(float64(got[index]-want[index])) > 1e-6 {
			t.Fatalf("normal matrix[%d] = %f, want %f", index, got[index], want[index])
		}
	}
}

func TestNormalMatrixIgnoresTranslationAndHandlesSingularMatrices(t *testing.T) {
	translated := gltf.Mat4{2, 0, 0, 0, 0, 4, 0, 0, 0, 0, 8, 0, 100, 200, 300, 1}
	got := normalMatrix(translated)
	want := [9]float32{0.5, 0, 0, 0, 0.25, 0, 0, 0, 0.125}
	if got != want {
		t.Fatalf("translated normal matrix = %v", got)
	}
	singular := gltf.Mat4{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	if got := normalMatrix(singular); got != [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1} {
		t.Fatalf("singular normal matrix = %v", got)
	}
}

func TestNormalMatrixKeepsExtremeInverseFinite(t *testing.T) {
	matrix := gltf.Mat4{math.SmallestNonzeroFloat32, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1}
	got := normalMatrix(matrix)
	for index, value := range got {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			t.Fatalf("normal matrix[%d] is non-finite: %v", index, got)
		}
	}
	if got[0] != 1 {
		t.Fatalf("scaled extreme inverse = %v", got)
	}
}

func TestFitToViewAvoidsFloat32IntermediateOverflow(t *testing.T) {
	fit := fitToView(gltf.Bounds{
		Min:   [3]float32{-math.MaxFloat32, -1, -1},
		Max:   [3]float32{math.MaxFloat32, 1, 1},
		Valid: true,
	})
	for index, value := range fit {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			t.Fatalf("fit matrix %d is non-finite: %v", index, fit)
		}
	}
}

func transformTestPoint(matrix gltf.Mat4, point [3]float32) [3]float32 {
	return [3]float32{
		matrix[0]*point[0] + matrix[4]*point[1] + matrix[8]*point[2] + matrix[12],
		matrix[1]*point[0] + matrix[5]*point[1] + matrix[9]*point[2] + matrix[13],
		matrix[2]*point[0] + matrix[6]*point[1] + matrix[10]*point[2] + matrix[14],
	}
}

func assertRendererMatrixClose(t *testing.T, got, want gltf.Mat4, tolerance float64) {
	t.Helper()
	for index := range got {
		if math.Abs(float64(got[index]-want[index])) > tolerance {
			t.Fatalf("matrix[%d] = %v, want %v; got %v", index, got[index], want[index], got)
		}
	}
}

func assertPointClose(t *testing.T, got, want [3]float32, tolerance float64) {
	t.Helper()
	for index := range got {
		if math.Abs(float64(got[index]-want[index])) > tolerance {
			t.Fatalf("point[%d] = %v, want %v; got %v", index, got[index], want[index], got)
		}
	}
}
