//go:build js && wasm

package renderer

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"syscall/js"
)

type GLB struct {
	JSON []byte
	BIN  []byte
}

type Accessor struct {
    BufferView    int    `json:"bufferView"`
    ComponentType int    `json:"componentType"`
    Count         int    `json:"count"`
    Type          string `json:"type"`
    ByteOffset    int    `json:"byteOffset"`
    Normalized    bool   `json:"normalized"`
}

type BufferView struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type Primitive struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   int            `json:"material"`
}

type MeshDef struct {
	Primitives []Primitive `json:"primitives"`
}

type Image struct {
	BufferView int    `json:"bufferView"`
	MimeType   string `json:"mimeType"`
	URI        string `json:"uri"`
}

type Texture struct {
	Source  int `json:"source"`
	Sampler int `json:"sampler"`
}

type TextureInfo struct {
	Index int `json:"index"`
}

type Sampler struct {
	MagFilter int `json:"magFilter"`
	MinFilter int `json:"minFilter"`
	WrapS     int `json:"wrapS"`
	WrapT     int `json:"wrapT"`
}

type PBRMetallicRoughness struct {
	BaseColorTexture TextureInfo `json:"baseColorTexture"`
}

type Material struct {
	PBRMetallicRoughness PBRMetallicRoughness `json:"pbrMetallicRoughness"`
}

type Gltf struct {
	Accessors   []Accessor   `json:"accessors"`
	BufferViews []BufferView `json:"bufferViews"`
	Meshes      []MeshDef    `json:"meshes"`

	Images    []Image    `json:"images"`
	Textures  []Texture  `json:"textures"`
	Materials []Material `json:"materials"`
	Samplers  []Sampler  `json:"samplers"`
}

type PrimitiveMesh struct {
    Positions []float32
    Normals   []float32
    UVs       []float32
    Indices   []uint32

    Texture    js.Value
    HasTexture bool
}

type Mesh struct {
    Primitives []PrimitiveMesh
}

func ParseGLB(data []byte) (*GLB, error) {

	reader := bytes.NewReader(data)

	var magic uint32
	var version uint32
	var length uint32

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&magic,
	); err != nil {
		return nil, err
	}

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&version,
	); err != nil {
		return nil, err
	}

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&length,
	); err != nil {
		return nil, err
	}

	if magic != 0x46546C67 {
		return nil, errors.New("invalid glb magic")
	}

	if version != 2 {
		return nil, errors.New("unsupported glb version")
	}

	var jsonChunkLength uint32
	var jsonChunkType uint32

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&jsonChunkLength,
	); err != nil {
		return nil, err
	}

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&jsonChunkType,
	); err != nil {
		return nil, err
	}

	if jsonChunkType != 0x4E4F534A {
		return nil, errors.New("invalid json chunk")
	}

	jsonBytes := make([]byte, jsonChunkLength)

	if _, err := reader.Read(jsonBytes); err != nil {
		return nil, err
	}

	var binChunkLength uint32
	var binChunkType uint32

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&binChunkLength,
	); err != nil {
		return nil, err
	}

	if err := binary.Read(
		reader,
		binary.LittleEndian,
		&binChunkType,
	); err != nil {
		return nil, err
	}

	if binChunkType != 0x004E4942 {
		return nil, errors.New("invalid bin chunk")
	}

	binBytes := make([]byte, binChunkLength)

	if _, err := reader.Read(binBytes); err != nil {
		return nil, err
	}

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

func parseGLBMesh(data []byte) (*Mesh, error) {

    glb, err := ParseGLB(data)
    if err != nil {
        return nil, err
    }

    gltf, err := ParseGltf(glb)
    if err != nil {
        return nil, err
    }

    if len(gltf.Meshes) == 0 {
        return nil, errors.New("no meshes")
    }

    meshDef := gltf.Meshes[0]

    result := &Mesh{
        Primitives: []PrimitiveMesh{},
    }

    for _, primitive := range meshDef.Primitives {

        positionAccessor,
            ok := primitive.Attributes["POSITION"]

        if !ok {
            continue
        }

        positions, err :=
            readFloat32Accessor(
                gltf,
                glb,
                positionAccessor,
            )

        if err != nil {
            return nil, err
        }

        var normals []float32

        if normalAccessor,
            ok := primitive.Attributes["NORMAL"]; ok {

            normals, err =
                readFloat32Accessor(
                    gltf,
                    glb,
                    normalAccessor,
                )

            if err != nil {
                return nil, err
            }

        } else {

            normals = make(
                []float32,
                len(positions),
            )
        }

        var uvs []float32

        if uvAccessor,
            ok := primitive.Attributes["TEXCOORD_0"]; ok {

            uvs, err =
                readFloat32Accessor(
                    gltf,
                    glb,
                    uvAccessor,
                )

            if err != nil {
                return nil, err
            }

        } else {

            vertexCount := len(positions) / 3

            uvs = make(
                []float32,
                vertexCount*2,
            )
        }

        indices, err := readIndices(
            gltf,
            glb,
            primitive.Indices,
        )

        if err != nil {
            return nil, err
        }

        var texture js.Value
        hasTexture := false

        if primitive.Material >= 0 &&
            primitive.Material < len(gltf.Materials) {

            imageBytes,
                mimeType,
                sampler,
                err := extractTextureBytes(
                gltf,
                glb,
                primitive.Material,
            )

            if err == nil &&
                len(imageBytes) > 0 {

                texture = uploadTexture(
                    imageBytes,
                    mimeType,
                    sampler,
                )

                hasTexture = true
            }
        }

        result.Primitives =
            append(
                result.Primitives,
                PrimitiveMesh{
                    Positions: positions,
                    Normals: normals,
                    UVs: uvs,
                    Indices: indices,

                    Texture: texture,
                    HasTexture: hasTexture,
                },
            )
    }

    return result, nil
}

func readFloat32Accessor(
	gltf *Gltf,
	glb *GLB,
	accessorIndex int,
) ([]float32, error) {

	if accessorIndex >= len(gltf.Accessors) {
		return nil, errors.New("invalid accessor")
	}

	accessor := gltf.Accessors[accessorIndex]

	if accessor.BufferView >= len(gltf.BufferViews) {
		return nil, errors.New("invalid bufferview")
	}

	view := gltf.BufferViews[accessor.BufferView]

	componentCount := 1

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

	componentSize := 4

	switch accessor.ComponentType {

	case 5126: // FLOAT
		componentSize = 4

	case 5123: // UNSIGNED_SHORT
		componentSize = 2

	case 5121: // UNSIGNED_BYTE
		componentSize = 1

	default:
		return nil, errors.New("unsupported accessor component type")
	}

	stride := view.ByteStride

	if stride == 0 {
		stride = componentCount * componentSize
	}

	start := view.ByteOffset + accessor.ByteOffset

	result := make(
		[]float32,
		accessor.Count*componentCount,
	)

	for i := 0; i < accessor.Count; i++ {

		vertexOffset := start + i*stride

		for j := 0; j < componentCount; j++ {

			offset := vertexOffset + j*componentSize

			switch accessor.ComponentType {

			case 5126:

				if offset+4 > len(glb.BIN) {
					return nil, errors.New("float accessor out of bounds")
				}

				bits := binary.LittleEndian.Uint32(
					glb.BIN[offset : offset+4],
				)

				result[i*componentCount+j] =
					math.Float32frombits(bits)

			case 5123:

				if offset+2 > len(glb.BIN) {
					return nil, errors.New("ushort accessor out of bounds")
				}

				value := binary.LittleEndian.Uint16(
					glb.BIN[offset : offset+2],
				)

				// normalized ushort -> float
				result[i*componentCount+j] =
					float32(value) / 65535.0

			case 5121:

				if offset+1 > len(glb.BIN) {
					return nil, errors.New("ubyte accessor out of bounds")
				}

				value := glb.BIN[offset]

				// normalized ubyte -> float
				result[i*componentCount+j] =
					float32(value) / 255.0
			}
		}
	}

	return result, nil
}

func readIndices(
	gltf *Gltf,
	glb *GLB,
	accessorIndex int,
) ([]uint32, error) {

	if accessorIndex >= len(gltf.Accessors) {
		return nil, errors.New("invalid accessor")
	}

	accessor := gltf.Accessors[accessorIndex]

	view := gltf.BufferViews[accessor.BufferView]

	start := view.ByteOffset +
		accessor.ByteOffset

	result := make([]uint32, accessor.Count)

	switch accessor.ComponentType {

	case 5121:

		for i := 0; i < accessor.Count; i++ {

			offset := start + i

			if offset+1 > len(glb.BIN) {
				return nil, errors.New("index out of bounds")
			}

			result[i] = uint32(glb.BIN[offset])
		}

	case 5123:

		for i := 0; i < accessor.Count; i++ {

			offset := start + i*2

			if offset+2 > len(glb.BIN) {
				return nil, errors.New("index out of bounds")
			}

			result[i] = uint32(
				binary.LittleEndian.Uint16(
					glb.BIN[offset : offset+2],
				),
			)
		}

	case 5125:

		for i := 0; i < accessor.Count; i++ {

			offset := start + i*4

			if offset+4 > len(glb.BIN) {
				return nil, errors.New("index out of bounds")
			}

			result[i] =
				binary.LittleEndian.Uint32(
					glb.BIN[offset : offset+4],
				)
		}

	default:
		return nil, errors.New("unsupported index type")
	}

	return result, nil
}

func extractTextureBytes(
	gltf *Gltf,
	glb *GLB,
	materialIndex int,
) ([]byte, string, *Sampler, error) {

	material := gltf.Materials[materialIndex]

	textureIndex := material.
		PBRMetallicRoughness.
		BaseColorTexture.
		Index

	if textureIndex >= len(gltf.Textures) {
		return nil, "", nil, errors.New("invalid texture")
	}

	texture := gltf.Textures[textureIndex]

	if texture.Source >= len(gltf.Images) {
		return nil, "", nil, errors.New("invalid image")
	}

	image := gltf.Images[texture.Source]

	var sampler *Sampler

	if texture.Sampler >= 0 &&
		texture.Sampler < len(gltf.Samplers) {

		sampler = &gltf.Samplers[texture.Sampler]
	}

	if image.URI != "" {

		if strings.HasPrefix(
			image.URI,
			"data:",
		) {

			parts := strings.SplitN(
				image.URI,
				",",
				2,
			)

			if len(parts) != 2 {
				return nil, "", nil, errors.New("invalid data uri")
			}

			meta := parts[0]
			payload := parts[1]

			mime := "image/png"

			if strings.Contains(meta, "image/jpeg") {
				mime = "image/jpeg"
			}

			bytes, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return nil, "", nil, err
			}

			return bytes, mime, sampler, nil
		}
	}

	view := gltf.BufferViews[image.BufferView]

	start := view.ByteOffset
	end := start + view.ByteLength

	if end > len(glb.BIN) {
		return nil, "", nil, errors.New("texture out of bounds")
	}

	return glb.BIN[start:end],
		image.MimeType,
		sampler,
		nil
}

func uploadTexture(
	imageBytes []byte,
	mimeType string,
	sampler *Sampler,
) js.Value {

	texture := GL.Call("createTexture")

	uint8Array := js.Global().
		Get("Uint8Array").
		New(len(imageBytes))

	js.CopyBytesToJS(
		uint8Array,
		imageBytes,
	)

	blob := js.Global().
		Get("Blob").
		New(
			[]any{uint8Array},
			map[string]any{
				"type": mimeType,
			},
		)

	url := js.Global().
		Get("URL").
		Call(
			"createObjectURL",
			blob,
		)

	img := js.Global().
		Get("Image").
		New()

	var onload js.Func

	onload = js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		GL.Call(
			"bindTexture",
			GL.Get("TEXTURE_2D"),
			texture,
		)

		GL.Call(
			"pixelStorei",
			GL.Get("UNPACK_FLIP_Y_WEBGL"),
			true,
		)

		GL.Call(
			"texImage2D",
			GL.Get("TEXTURE_2D"),
			0,
			GL.Get("RGBA"),
			GL.Get("RGBA"),
			GL.Get("UNSIGNED_BYTE"),
			img,
		)

		minFilter := GL.Get("LINEAR_MIPMAP_LINEAR")
		magFilter := GL.Get("LINEAR")
		wrapS := GL.Get("REPEAT")
		wrapT := GL.Get("REPEAT")

		if sampler != nil {

			if sampler.MinFilter != 0 {
				minFilter = js.ValueOf(sampler.MinFilter)
			}

			if sampler.MagFilter != 0 {
				magFilter = js.ValueOf(sampler.MagFilter)
			}

			if sampler.WrapS != 0 {
				wrapS = js.ValueOf(sampler.WrapS)
			}

			if sampler.WrapT != 0 {
				wrapT = js.ValueOf(sampler.WrapT)
			}
		}

		GL.Call(
			"texParameteri",
			GL.Get("TEXTURE_2D"),
			GL.Get("TEXTURE_MIN_FILTER"),
			minFilter,
		)

		GL.Call(
			"texParameteri",
			GL.Get("TEXTURE_2D"),
			GL.Get("TEXTURE_MAG_FILTER"),
			magFilter,
		)

		GL.Call(
			"texParameteri",
			GL.Get("TEXTURE_2D"),
			GL.Get("TEXTURE_WRAP_S"),
			wrapS,
		)

		GL.Call(
			"texParameteri",
			GL.Get("TEXTURE_2D"),
			GL.Get("TEXTURE_WRAP_T"),
			wrapT,
		)

		GL.Call(
			"generateMipmap",
			GL.Get("TEXTURE_2D"),
		)

		js.Global().
			Get("URL").
			Call(
				"revokeObjectURL",
				url,
			)

		onload.Release()

		println("texture uploaded")

		return nil
	})

	img.Set("onload", onload)
	img.Set("src", url)

	return texture
}

func LoadGLBMeshAsync(
	path string,
	onLoaded func(*Mesh),
) {

	promise := js.Global().
		Call("fetch", path)

	var thenFunc js.Func
	var catchFunc js.Func

	thenFunc = js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		response := args[0]

		arrayBufferPromise := response.Call(
			"arrayBuffer",
		)

		var arrayBufferThen js.Func

		arrayBufferThen = js.FuncOf(func(
			this js.Value,
			args []js.Value,
		) any {

			buffer := args[0]

			uint8Array := js.Global().
				Get("Uint8Array").
				New(buffer)

			length := uint8Array.
				Get("length").
				Int()

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

			arrayBufferThen.Release()
			thenFunc.Release()
			catchFunc.Release()

			return nil
		})

		arrayBufferPromise.Call(
			"then",
			arrayBufferThen,
		)

		return nil
	})

	catchFunc = js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		println(
			"failed to load glb:",
			args[0].String(),
		)

		thenFunc.Release()
		catchFunc.Release()

		return nil
	})

	promise.Call("then", thenFunc)
	promise.Call("catch", catchFunc)
}