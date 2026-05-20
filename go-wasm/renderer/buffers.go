//go:build js && wasm

package renderer

import "syscall/js"

type GPUMesh struct {
	VertexBuffer js.Value
	IndexBuffer  js.Value

	Texture    js.Value
	HasTexture bool

	IndexCount int
}

func UploadPrimitiveMesh(
	mesh *PrimitiveMesh,
) *GPUMesh {

	vertexCount := len(mesh.Positions) / 3

	interleaved := make(
		[]float32,
		0,
		vertexCount*8,
	)

	for i := 0; i < vertexCount; i++ {

		px := float32(0)
		py := float32(0)
		pz := float32(0)

		nx := float32(0)
		ny := float32(0)
		nz := float32(1)

		u := float32(0)
		v := float32(0)

		// POSITION

		px = mesh.Positions[i*3+0]
		py = mesh.Positions[i*3+1]
		pz = mesh.Positions[i*3+2]

		// NORMAL

		nx = mesh.Normals[i*3+0]
		ny = mesh.Normals[i*3+1]
		nz = mesh.Normals[i*3+2]

		// UV

		u = mesh.UVs[i*2+0]
		v = mesh.UVs[i*2+1]

		interleaved = append(
			interleaved,

			// POSITION
			px,
			py,
			pz,

			// NORMAL
			nx,
			ny,
			nz,

			// UV
			u,
			v,
		)
	}

	println(
		"vertex count",
		vertexCount,
	)

	println(
		"interleaved len",
		len(interleaved),
	)

	vertexBuffer := GL.Call(
		"createBuffer",
	)

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

	indexBuffer := GL.Call(
		"createBuffer",
	)

	GL.Call(
		"bindBuffer",
		GL.Get("ELEMENT_ARRAY_BUFFER"),
		indexBuffer,
	)

	indexArray := js.Global().
		Get("Uint32Array").
		New(len(mesh.Indices))

	for i, v := range mesh.Indices {
		indexArray.SetIndex(
			i,
			v,
		)
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

		Texture:    mesh.Texture,
		HasTexture: mesh.HasTexture,

		IndexCount: len(mesh.Indices),
	}
}
