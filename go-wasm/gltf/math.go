package gltf

import (
	"fmt"
	"math"
)

func multiply(a, b Mat4) Mat4 {
	var out Mat4
	for column := 0; column < 4; column++ {
		for row := 0; row < 4; row++ {
			var value float32
			for k := 0; k < 4; k++ {
				value += a[k*4+row] * b[column*4+k]
			}
			out[column*4+row] = value
		}
	}
	return out
}

func nodeMatrix(node nodeDef) (Mat4, error) {
	if len(node.Matrix) != 0 {
		if len(node.Translation) != 0 || len(node.Rotation) != 0 || len(node.Scale) != 0 {
			return Mat4{}, fmt.Errorf("node cannot define both matrix and TRS transforms")
		}
		if len(node.Matrix) != 16 {
			return Mat4{}, fmt.Errorf("node matrix must contain 16 values")
		}
		var result Mat4
		for i, value := range node.Matrix {
			if !representableFloat32(value) {
				return Mat4{}, fmt.Errorf("node matrix contains a value outside the supported float range")
			}
			result[i] = float32(value)
		}
		return result, nil
	}

	translation := [3]float64{0, 0, 0}
	rotation := [4]float64{0, 0, 0, 1}
	scale := [3]float64{1, 1, 1}
	if err := copyVector("translation", translation[:], node.Translation); err != nil {
		return Mat4{}, err
	}
	if err := copyVector("rotation", rotation[:], node.Rotation); err != nil {
		return Mat4{}, err
	}
	if err := copyVector("scale", scale[:], node.Scale); err != nil {
		return Mat4{}, err
	}

	length := math.Sqrt(rotation[0]*rotation[0] + rotation[1]*rotation[1] + rotation[2]*rotation[2] + rotation[3]*rotation[3])
	if length == 0 || !finite(length) {
		return Mat4{}, fmt.Errorf("node rotation quaternion is invalid")
	}
	x, y, z, w := rotation[0]/length, rotation[1]/length, rotation[2]/length, rotation[3]/length
	xx, yy, zz := x*x, y*y, z*z
	xy, xz, yz := x*y, x*z, y*z
	wx, wy, wz := w*x, w*y, w*z

	return Mat4{
		float32((1 - 2*(yy+zz)) * scale[0]), float32((2 * (xy + wz)) * scale[0]), float32((2 * (xz - wy)) * scale[0]), 0,
		float32((2 * (xy - wz)) * scale[1]), float32((1 - 2*(xx+zz)) * scale[1]), float32((2 * (yz + wx)) * scale[1]), 0,
		float32((2 * (xz + wy)) * scale[2]), float32((2 * (yz - wx)) * scale[2]), float32((1 - 2*(xx+yy)) * scale[2]), 0,
		float32(translation[0]), float32(translation[1]), float32(translation[2]), 1,
	}, nil
}

func copyVector(name string, destination, source []float64) error {
	if len(source) == 0 {
		return nil
	}
	if len(source) != len(destination) {
		return fmt.Errorf("node %s must contain %d values", name, len(destination))
	}
	for i, value := range source {
		if !representableFloat32(value) {
			return fmt.Errorf("node %s contains a value outside the supported float range", name)
		}
		destination[i] = value
	}
	return nil
}

func transformPoint(matrix Mat4, x, y, z float32) ([3]float32, error) {
	w := matrix[3]*x + matrix[7]*y + matrix[11]*z + matrix[15]
	if w == 0 {
		return [3]float32{}, fmt.Errorf("node transform produces a zero homogeneous coordinate")
	}
	point := [3]float32{
		(matrix[0]*x + matrix[4]*y + matrix[8]*z + matrix[12]) / w,
		(matrix[1]*x + matrix[5]*y + matrix[9]*z + matrix[13]) / w,
		(matrix[2]*x + matrix[6]*y + matrix[10]*z + matrix[14]) / w,
	}
	for _, value := range point {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return [3]float32{}, fmt.Errorf("node transform produces a non-finite position")
		}
	}
	return point, nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func representableFloat32(value float64) bool {
	return finite(value) && math.Abs(value) <= math.MaxFloat32
}
