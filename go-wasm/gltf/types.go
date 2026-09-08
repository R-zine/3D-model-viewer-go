// Package gltf parses the deliberately supported, renderable subset of GLB 2.0.
// It has no browser or GPU dependencies so it can be unit-tested and fuzzed on
// the host platform.
package gltf

// Limits bounds all allocations controlled by uploaded model data.
type Limits struct {
	MaxFileBytes         int
	MaxJSONBytes         int
	MaxTextureBytes      int
	MaxTextureBytesTotal int
	MaxTextures          int
	MaxAccessorCount     int
	MaxVertices          int
	MaxIndices           int
	MaxPrimitives        int
	MaxNodes             int
	MaxNodeDepth         int
}

func DefaultLimits() Limits {
	return Limits{
		MaxFileBytes:         128 << 20,
		MaxJSONBytes:         16 << 20,
		MaxTextureBytes:      32 << 20,
		MaxTextureBytesTotal: 64 << 20,
		MaxTextures:          256,
		MaxAccessorCount:     3_000_000,
		MaxVertices:          1_000_000,
		MaxIndices:           3_000_000,
		MaxPrimitives:        10_000,
		MaxNodes:             100_000,
		MaxNodeDepth:         256,
	}
}

type Mat4 [16]float32

type Bounds struct {
	Min   [3]float32
	Max   [3]float32
	Valid bool
}

type Sampler struct {
	MagFilter int
	MinFilter int
	WrapS     int
	WrapT     int
}

type Texture struct {
	Data     []byte
	MIMEType string
	Sampler  Sampler
}

type Material struct {
	BaseColorFactor  [4]float32
	BaseColorTexture int // -1 means the renderer's white fallback texture.
	DoubleSided      bool
	AlphaMode        string
	AlphaCutoff      float32
}

type Primitive struct {
	Positions []float32
	Normals   []float32
	UVs       []float32
	Indices   []uint32
	Mode      uint32
	Transform Mat4
	Material  Material
}

type Model struct {
	Primitives []Primitive
	Textures   []Texture
	Bounds     Bounds
}

func Identity() Mat4 {
	return Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
}
