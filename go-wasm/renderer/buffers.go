//go:build js && wasm

package renderer

import "syscall/js"

type GPUMesh struct {
	VertexBuffer js.Value
	IndexBuffer  js.Value
	IndexCount   int
}

func UploadMesh(mesh *Mesh) *GPUMesh {

	interleaved := make([]float32, 0)

	vertexCount := len(mesh.Positions) / 3

	for i := 0; i < vertexCount; i++ {

		interleaved = append(interleaved,
			mesh.Positions[i*3],
			mesh.Positions[i*3+1],
			mesh.Positions[i*3+2],

			mesh.Normals[i*3],
			mesh.Normals[i*3+1],
			mesh.Normals[i*3+2],
		)
	}

	vertexBuffer := GL.Call("createBuffer")

	GL.Call(
		"bindBuffer",
		GL.Get("ARRAY_BUFFER"),
		vertexBuffer,
	)

	vertexArray := js.Global().
		Get("Float32Array").
		New(len(interleaved))

	for i, v := range interleaved {
		vertexArray.SetIndex(i, v)
	}

	GL.Call(
		"bufferData",
		GL.Get("ARRAY_BUFFER"),
		vertexArray,
		GL.Get("STATIC_DRAW"),
	)

	indexBuffer := GL.Call("createBuffer")

	GL.Call(
		"bindBuffer",
		GL.Get("ELEMENT_ARRAY_BUFFER"),
		indexBuffer,
	)

	indexArray := js.Global().
		Get("Uint32Array").
		New(len(mesh.Indices))

	for i, v := range mesh.Indices {
		indexArray.SetIndex(i, v)
	}

	GL.Call(
		"bufferData",
		GL.Get("ELEMENT_ARRAY_BUFFER"),
		indexArray,
		GL.Get("STATIC_DRAW"),
	)

	return &GPUMesh{
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   len(mesh.Indices),
	}
}