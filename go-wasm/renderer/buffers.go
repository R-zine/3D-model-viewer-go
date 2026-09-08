//go:build js && wasm

package renderer

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
	"viewer/gltf"
)

type GPUMesh struct {
	VertexArray  js.Value
	VertexBuffer js.Value
	IndexBuffer  js.Value
	IndexCount   int
	Mode         uint32
	Transform    gltf.Mat4
	Material     gltf.Material
}

type GPUModel struct {
	Meshes   []*GPUMesh
	Textures []*GPUTexture
}

func (model *GPUModel) Dispose(gl js.Value) {
	if model == nil {
		return
	}
	for _, mesh := range model.Meshes {
		if mesh.VertexArray.Truthy() {
			gl.Call("deleteVertexArray", mesh.VertexArray)
		}
		if mesh.VertexBuffer.Truthy() {
			gl.Call("deleteBuffer", mesh.VertexBuffer)
		}
		if mesh.IndexBuffer.Truthy() {
			gl.Call("deleteBuffer", mesh.IndexBuffer)
		}
	}
	for _, texture := range model.Textures {
		texture.Dispose(gl)
	}
	model.Meshes = nil
	model.Textures = nil
}

func (renderer *Renderer) uploadModel(source *gltf.Model) (result *GPUModel, err error) {
	result = &GPUModel{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.Dispose(renderer.gl)
			result = nil
			err = fmt.Errorf("upload model to WebGL: %v", recovered)
		}
	}()

	for index, texture := range source.Textures {
		gpuTexture, textureErr := renderer.uploadTexture(texture, index)
		if textureErr != nil {
			result.Dispose(renderer.gl)
			return nil, textureErr
		}
		result.Textures = append(result.Textures, gpuTexture)
	}
	for index := range source.Primitives {
		mesh, meshErr := renderer.uploadPrimitive(&source.Primitives[index])
		if meshErr != nil {
			result.Dispose(renderer.gl)
			return nil, meshErr
		}
		result.Meshes = append(result.Meshes, mesh)
	}
	return result, nil
}

func (renderer *Renderer) uploadPrimitive(primitive *gltf.Primitive) (*GPUMesh, error) {
	vertexCount := len(primitive.Positions) / 3
	if len(primitive.Normals) != vertexCount*3 || len(primitive.UVs) != vertexCount*2 {
		return nil, fmt.Errorf("primitive attribute counts do not match")
	}
	vertexBytes := make([]byte, vertexCount*8*4)
	for vertex := 0; vertex < vertexCount; vertex++ {
		values := [8]float32{
			primitive.Positions[vertex*3],
			primitive.Positions[vertex*3+1],
			primitive.Positions[vertex*3+2],
			primitive.Normals[vertex*3],
			primitive.Normals[vertex*3+1],
			primitive.Normals[vertex*3+2],
			primitive.UVs[vertex*2],
			primitive.UVs[vertex*2+1],
		}
		for component, value := range values {
			offset := (vertex*8 + component) * 4
			binary.LittleEndian.PutUint32(vertexBytes[offset:offset+4], math.Float32bits(value))
		}
	}
	indexBytes := make([]byte, len(primitive.Indices)*4)
	for index, value := range primitive.Indices {
		binary.LittleEndian.PutUint32(indexBytes[index*4:index*4+4], value)
	}

	gl := renderer.gl
	vertexArray := gl.Call("createVertexArray")
	vertexBuffer := gl.Call("createBuffer")
	indexBuffer := gl.Call("createBuffer")
	if vertexArray.IsNull() || vertexBuffer.IsNull() || indexBuffer.IsNull() {
		if vertexArray.Truthy() {
			gl.Call("deleteVertexArray", vertexArray)
		}
		if vertexBuffer.Truthy() {
			gl.Call("deleteBuffer", vertexBuffer)
		}
		if indexBuffer.Truthy() {
			gl.Call("deleteBuffer", indexBuffer)
		}
		return nil, fmt.Errorf("WebGL could not allocate primitive buffers")
	}
	gl.Call("bindVertexArray", vertexArray)
	gl.Call("bindBuffer", gl.Get("ARRAY_BUFFER"), vertexBuffer)
	gl.Call("bufferData", gl.Get("ARRAY_BUFFER"), bytesToJS(vertexBytes), gl.Get("STATIC_DRAW"))
	gl.Call("bindBuffer", gl.Get("ELEMENT_ARRAY_BUFFER"), indexBuffer)
	gl.Call("bufferData", gl.Get("ELEMENT_ARRAY_BUFFER"), bytesToJS(indexBytes), gl.Get("STATIC_DRAW"))

	stride := 8 * 4
	gl.Call("vertexAttribPointer", renderer.locations.position, 3, gl.Get("FLOAT"), false, stride, 0)
	gl.Call("vertexAttribPointer", renderer.locations.normal, 3, gl.Get("FLOAT"), false, stride, 12)
	gl.Call("vertexAttribPointer", renderer.locations.uv, 2, gl.Get("FLOAT"), false, stride, 24)
	gl.Call("enableVertexAttribArray", renderer.locations.position)
	gl.Call("enableVertexAttribArray", renderer.locations.normal)
	gl.Call("enableVertexAttribArray", renderer.locations.uv)
	gl.Call("bindVertexArray", nil)

	return &GPUMesh{
		VertexArray:  vertexArray,
		VertexBuffer: vertexBuffer,
		IndexBuffer:  indexBuffer,
		IndexCount:   len(primitive.Indices),
		Mode:         primitive.Mode,
		Transform:    primitive.Transform,
		Material:     primitive.Material,
	}, nil
}

func bytesToJS(data []byte) js.Value {
	array := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(array, data)
	return array
}
