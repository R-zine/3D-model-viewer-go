package renderer

import (
	"math"
	"viewer/gltf"
)

func perspective(fieldOfView, aspect, near, far float64) gltf.Mat4 {
	f := float32(1 / math.Tan(fieldOfView*math.Pi/360))
	return gltf.Mat4{
		f / float32(aspect), 0, 0, 0,
		0, f, 0, 0,
		0, 0, float32((far + near) / (near - far)), -1,
		0, 0, float32((2 * far * near) / (near - far)), 0,
	}
}

func fitToView(bounds gltf.Bounds) gltf.Mat4 {
	if !bounds.Valid {
		return gltf.Identity()
	}
	center := [3]float64{
		(float64(bounds.Min[0]) + float64(bounds.Max[0])) / 2,
		(float64(bounds.Min[1]) + float64(bounds.Max[1])) / 2,
		(float64(bounds.Min[2]) + float64(bounds.Max[2])) / 2,
	}
	extent := max(float64(bounds.Max[0])-float64(bounds.Min[0]), max(float64(bounds.Max[1])-float64(bounds.Min[1]), float64(bounds.Max[2])-float64(bounds.Min[2])))
	scale := 1.0
	if extent > 0 {
		scale = 2 / extent
	}
	return gltf.Mat4{
		float32(scale), 0, 0, 0,
		0, float32(scale), 0, 0,
		0, 0, float32(scale), 0,
		float32(-center[0] * scale), float32(-center[1] * scale), float32(-center[2] * scale), 1,
	}
}

func rotationMatrix(x, y float64) gltf.Mat4 {
	cx, sx := float32(math.Cos(x)), float32(math.Sin(x))
	cy, sy := float32(math.Cos(y)), float32(math.Sin(y))
	rotationX := gltf.Mat4{1, 0, 0, 0, 0, cx, sx, 0, 0, -sx, cx, 0, 0, 0, 0, 1}
	rotationY := gltf.Mat4{cy, 0, -sy, 0, 0, 1, 0, 0, sy, 0, cy, 0, 0, 0, 0, 1}
	return multiplyMatrices(rotationY, rotationX)
}

func multiplyMatrices(a, b gltf.Mat4) gltf.Mat4 {
	var result gltf.Mat4
	for column := 0; column < 4; column++ {
		for row := 0; row < 4; row++ {
			for k := 0; k < 4; k++ {
				result[column*4+row] += a[k*4+row] * b[column*4+k]
			}
		}
	}
	return result
}

func normalMatrix(matrix gltf.Mat4) [9]float32 {
	a, b, c := float64(matrix[0]), float64(matrix[4]), float64(matrix[8])
	d, e, f := float64(matrix[1]), float64(matrix[5]), float64(matrix[9])
	g, h, i := float64(matrix[2]), float64(matrix[6]), float64(matrix[10])
	determinant := a*(e*i-f*h) - b*(d*i-f*g) + c*(d*h-e*g)
	if determinant == 0 || math.IsNaN(determinant) || math.IsInf(determinant, 0) {
		return [9]float32{1, 0, 0, 0, 1, 0, 0, 0, 1}
	}
	inverse := 1 / determinant
	values := [9]float64{
		(e*i - f*h) * inverse, (c*h - b*i) * inverse, (b*f - c*e) * inverse,
		(f*g - d*i) * inverse, (a*i - c*g) * inverse, (c*d - a*f) * inverse,
		(d*h - e*g) * inverse, (b*g - a*h) * inverse, (a*e - b*d) * inverse,
	}
	maxValue := 0.0
	for _, value := range values {
		maxValue = max(maxValue, math.Abs(value))
	}
	scale := 1.0
	if maxValue > math.MaxFloat32 {
		scale = 1 / maxValue
	}
	var result [9]float32
	for index, value := range values {
		result[index] = float32(value * scale)
	}
	return result
}
