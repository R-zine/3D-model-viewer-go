package renderer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"syscall/js"
)

type GLB struct {
	JSON []byte
	BIN  []byte
}

type Gltf struct {
	Accessors   []Accessor   `json:"accessors"`
	BufferViews []BufferView `json:"bufferViews"`
	Meshes      []MeshDef    `json:"meshes"`
}

type Accessor struct {
	BufferView    int    `json:"bufferView"`
	ComponentType int    `json:"componentType"`
	Count         int    `json:"count"`
	Type          string `json:"type"`
	ByteOffset    int    `json:"byteOffset"`
}

type BufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type MeshDef struct {
	Primitives []Primitive `json:"primitives"`
}

type Primitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
}

type Mesh struct {
	Positions []float32
	Normals   []float32
	Indices []uint32
}

func fetchBytes(path string) ([]byte, error) {

	done := make(chan struct{})

	var result []byte
	var fetchErr error

	promise := js.Global().Call(
		"fetch",
		path,
	)

	thenFunc := js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		response := args[0]

		arrayBufferPromise := response.Call(
			"arrayBuffer",
		)

		arrayBufferThen := js.FuncOf(func(
			this js.Value,
			args []js.Value,
		) any {

			buffer := args[0]

			uint8Array := js.Global().
				Get("Uint8Array").
				New(buffer)

			length := uint8Array.Get(
				"length",
			).Int()

			result = make([]byte, length)

			js.CopyBytesToGo(
				result,
				uint8Array,
			)

			close(done)

			return nil
		})

		arrayBufferPromise.Call(
			"then",
			arrayBufferThen,
		)

		return nil
	})

	catchFunc := js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		fetchErr = errors.New(
			args[0].String(),
		)

		close(done)

		return nil
	})

	promise.Call("then", thenFunc)
	promise.Call("catch", catchFunc)

	<-done

	return result, fetchErr
}

func parseGLBMesh(
	data []byte,
) (*Mesh, error) {

	glb, err := ParseGLB(data)
	if err != nil {
		return nil, err
	}

	var gltf Gltf

	err = json.Unmarshal(
		glb.JSON,
		&gltf,
	)

	if err != nil {
		return nil, err
	}

	if len(gltf.Meshes) == 0 {
		return nil, errors.New(
			"no meshes in glb",
		)
	}

	meshDef := gltf.Meshes[0]

	if len(meshDef.Primitives) == 0 {
		return nil, errors.New(
			"no primitives in mesh",
		)
	}

	primitive := meshDef.Primitives[0]

	positionAccessor,
	ok := primitive.Attributes["POSITION"]

	if !ok {
		return nil, errors.New(
			"mesh missing POSITION",
		)
	}

	normalAccessor,
	hasNormals := primitive.Attributes["NORMAL"]

	positions := readFloat32Accessor(
		&gltf,
		glb,
		positionAccessor,
	)

	var normals []float32

	if hasNormals {

		normals = readFloat32Accessor(
			&gltf,
			glb,
			normalAccessor,
		)

	} else {

		normals = make(
			[]float32,
			len(positions),
		)
	}

	indices := readIndices(
		&gltf,
		glb,
		primitive.Indices,
	)

	return &Mesh{
		Positions: positions,
		Normals:   normals,
		Indices:   indices,
	}, nil
}

func ParseGLB(data []byte) (*GLB, error) {

	reader := bytes.NewReader(data)

	var magic uint32
	var version uint32
	var length uint32

	binary.Read(
		reader,
		binary.LittleEndian,
		&magic,
	)

	binary.Read(
		reader,
		binary.LittleEndian,
		&version,
	)

	binary.Read(
		reader,
		binary.LittleEndian,
		&length,
	)

	if magic != 0x46546C67 {
		return nil, errors.New(
			"invalid glb",
		)
	}

	var jsonChunkLength uint32
	var jsonChunkType uint32

	binary.Read(
		reader,
		binary.LittleEndian,
		&jsonChunkLength,
	)

	binary.Read(
		reader,
		binary.LittleEndian,
		&jsonChunkType,
	)

	jsonBytes := make(
		[]byte,
		jsonChunkLength,
	)

	reader.Read(jsonBytes)

	var binChunkLength uint32
	var binChunkType uint32

	binary.Read(
		reader,
		binary.LittleEndian,
		&binChunkLength,
	)

	binary.Read(
		reader,
		binary.LittleEndian,
		&binChunkType,
	)

	binBytes := make(
		[]byte,
		binChunkLength,
	)

	reader.Read(binBytes)

	return &GLB{
		JSON: jsonBytes,
		BIN:  binBytes,
	}, nil
}

func ParseGltf(glb *GLB) (*Gltf, error) {

	var gltf Gltf

	err := json.Unmarshal(glb.JSON, &gltf)
	if err != nil {
		return nil, err
	}

	return &gltf, nil
}

func readFloat32Accessor(
	gltf *Gltf,
	glb *GLB,
	accessorIndex int,
) []float32 {

	accessor := gltf.Accessors[accessorIndex]

	view := gltf.BufferViews[accessor.BufferView]

	start := view.ByteOffset +
		accessor.ByteOffset

	componentCount := 3

	switch accessor.Type {

	case "SCALAR":
		componentCount = 1

	case "VEC2":
		componentCount = 2

	case "VEC3":
		componentCount = 3

	case "VEC4":
		componentCount = 4
	}

	stride := view.ByteStride

	if stride == 0 {
		stride = componentCount * 4
	}

	result := make(
		[]float32,
		accessor.Count*componentCount,
	)

	for i := 0; i < accessor.Count; i++ {

		vertexOffset := start + i*stride

		for j := 0; j < componentCount; j++ {

			offset := vertexOffset + j*4

			bits := binary.LittleEndian.Uint32(
				glb.BIN[offset : offset+4],
			)

			result[i*componentCount+j] = math.Float32frombits(bits)
		}
	}

	return result
}

func readIndices(
	gltf *Gltf,
	glb *GLB,
	accessorIndex int,
) []uint32 {

	accessor := gltf.Accessors[	accessorIndex]

	view := gltf.BufferViews[accessor.BufferView]

	start := view.ByteOffset +
		accessor.ByteOffset

	result := make(
		[]uint32,
		accessor.Count,
	)

	switch accessor.ComponentType {

	case 5123:

		for i := 0; i < accessor.Count; i++ {

			offset := start + i*2

			result[i] = uint32(
				binary.LittleEndian.Uint16(
					glb.BIN[offset : offset+2],
				),
			)
		}

	case 5125:

		for i := 0; i < accessor.Count; i++ {

			offset := start + i*4

			result[i] =
				binary.LittleEndian.Uint32(
					glb.BIN[offset : offset+4],
				)
		}
	}

	return result
}

func LoadGLBMeshAsync(
	path string,
	onLoaded func(*Mesh),
) {

	promise := js.Global().Call(
		"fetch",
		path,
	)

	thenFunc := js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		response := args[0]

		arrayBufferPromise := response.Call(
			"arrayBuffer",
		)

		arrayBufferThen := js.FuncOf(func(
			this js.Value,
			args []js.Value,
		) any {

			buffer := args[0]

			uint8Array := js.Global().
				Get("Uint8Array").
				New(buffer)

			length := uint8Array.Get(
				"length",
			).Int()

			data := make([]byte, length)

			js.CopyBytesToGo(
				data,
				uint8Array,
			)

			mesh, err := parseGLBMesh(data)
			if err != nil {
				println(err.Error())
				return nil
			}

			onLoaded(mesh)

			return nil
		})

		arrayBufferPromise.Call(
			"then",
			arrayBufferThen,
		)

		return nil
	})

	promise.Call("then", thenFunc)
}