package gltf

import "encoding/json"

type document struct {
	Asset              assetDef          `json:"asset"`
	ExtensionsRequired []string          `json:"extensionsRequired"`
	Scene              *int              `json:"scene"`
	Scenes             []sceneDef        `json:"scenes"`
	Nodes              []nodeDef         `json:"nodes"`
	Meshes             []meshDef         `json:"meshes"`
	Buffers            []bufferDef       `json:"buffers"`
	BufferViews        []bufferViewDef   `json:"bufferViews"`
	Accessors          []accessorDef     `json:"accessors"`
	Images             []imageDef        `json:"images"`
	Textures           []textureDef      `json:"textures"`
	Materials          []materialDef     `json:"materials"`
	Samplers           []samplerDef      `json:"samplers"`
	Skins              []json.RawMessage `json:"skins"`
}

type assetDef struct {
	Version string `json:"version"`
}

type sceneDef struct {
	Nodes []int `json:"nodes"`
}

type nodeDef struct {
	Mesh        *int      `json:"mesh"`
	Skin        *int      `json:"skin"`
	Children    []int     `json:"children"`
	Matrix      []float64 `json:"matrix"`
	Translation []float64 `json:"translation"`
	Rotation    []float64 `json:"rotation"`
	Scale       []float64 `json:"scale"`
}

type meshDef struct {
	Primitives []primitiveDef `json:"primitives"`
}

type primitiveDef struct {
	Attributes map[string]int   `json:"attributes"`
	Indices    *int             `json:"indices"`
	Material   *int             `json:"material"`
	Mode       *uint32          `json:"mode"`
	Targets    []map[string]int `json:"targets"`
}

type bufferDef struct {
	URI        string `json:"uri"`
	ByteLength int    `json:"byteLength"`
}

type bufferViewDef struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	ByteStride int `json:"byteStride"`
}

type accessorDef struct {
	BufferView    *int            `json:"bufferView"`
	ComponentType int             `json:"componentType"`
	Count         int             `json:"count"`
	Type          string          `json:"type"`
	ByteOffset    int             `json:"byteOffset"`
	Normalized    bool            `json:"normalized"`
	Sparse        *accessorSparse `json:"sparse"`
}

type accessorSparse struct {
	Count int `json:"count"`
}

type imageDef struct {
	BufferView *int   `json:"bufferView"`
	MIMEType   string `json:"mimeType"`
	URI        string `json:"uri"`
}

type textureDef struct {
	Source  *int `json:"source"`
	Sampler *int `json:"sampler"`
}

type textureInfoDef struct {
	Index    *int `json:"index"`
	TexCoord int  `json:"texCoord"`
}

type pbrDef struct {
	BaseColorFactor  []float64       `json:"baseColorFactor"`
	BaseColorTexture *textureInfoDef `json:"baseColorTexture"`
}

type materialDef struct {
	PBR         *pbrDef  `json:"pbrMetallicRoughness"`
	DoubleSided bool     `json:"doubleSided"`
	AlphaMode   string   `json:"alphaMode"`
	AlphaCutoff *float64 `json:"alphaCutoff"`
}

type samplerDef struct {
	MagFilter *int `json:"magFilter"`
	MinFilter *int `json:"minFilter"`
	WrapS     *int `json:"wrapS"`
	WrapT     *int `json:"wrapT"`
}
