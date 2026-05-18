//go:build js && wasm

package renderer

import "syscall/js"

var (
	positionBuffer js.Value
	indexBuffer    js.Value

	indexCount int
)

func createCubeMesh() {

	vertices := []float32{

		// FRONT
		-0.5, -0.5, 0.5, 0, 0, 1,
		0.5, -0.5, 0.5, 0, 0, 1,
		0.5, 0.5, 0.5, 0, 0, 1,
		-0.5, 0.5, 0.5, 0, 0, 1,

		// BACK
		0.5, -0.5, -0.5, 0, 0, -1,
		-0.5, -0.5, -0.5, 0, 0, -1,
		-0.5, 0.5, -0.5, 0, 0, -1,
		0.5, 0.5, -0.5, 0, 0, -1,

		// LEFT
		-0.5, -0.5, -0.5, -1, 0, 0,
		-0.5, -0.5, 0.5, -1, 0, 0,
		-0.5, 0.5, 0.5, -1, 0, 0,
		-0.5, 0.5, -0.5, -1, 0, 0,

		// RIGHT
		0.5, -0.5, 0.5, 1, 0, 0,
		0.5, -0.5, -0.5, 1, 0, 0,
		0.5, 0.5, -0.5, 1, 0, 0,
		0.5, 0.5, 0.5, 1, 0, 0,

		// TOP
		-0.5, 0.5, 0.5, 0, 1, 0,
		0.5, 0.5, 0.5, 0, 1, 0,
		0.5, 0.5, -0.5, 0, 1, 0,
		-0.5, 0.5, -0.5, 0, 1, 0,

		// BOTTOM
		-0.5, -0.5, -0.5, 0, -1, 0,
		0.5, -0.5, -0.5, 0, -1, 0,
		0.5, -0.5, 0.5, 0, -1, 0,
		-0.5, -0.5, 0.5, 0, -1, 0,
	}

	indices := []uint16{
		0, 1, 2, 2, 3, 0,
		4, 5, 6, 6, 7, 4,
		8, 9, 10, 10, 11, 8,
		12, 13, 14, 14, 15, 12,
		16, 17, 18, 18, 19, 16,
		20, 21, 22, 22, 23, 20,
	}

	indexCount = len(indices)

	jsVertices := js.Global().
		Get("Float32Array").
		New(len(vertices))

	for i, v := range vertices {
		jsVertices.SetIndex(i, v)
	}

	jsIndices := js.Global().
		Get("Uint16Array").
		New(len(indices))

	for i, v := range indices {
		jsIndices.SetIndex(i, v)
	}

	positionBuffer = GL.Call(
		"createBuffer",
	)

	GL.Call(
		"bindBuffer",
		GL.Get("ARRAY_BUFFER"),
		positionBuffer,
	)

	GL.Call(
		"bufferData",
		GL.Get("ARRAY_BUFFER"),
		jsVertices,
		GL.Get("STATIC_DRAW"),
	)

	indexBuffer = GL.Call(
		"createBuffer",
	)

	GL.Call(
		"bindBuffer",
		GL.Get("ELEMENT_ARRAY_BUFFER"),
		indexBuffer,
	)

	GL.Call(
		"bufferData",
		GL.Get("ELEMENT_ARRAY_BUFFER"),
		jsIndices,
		GL.Get("STATIC_DRAW"),
	)
}