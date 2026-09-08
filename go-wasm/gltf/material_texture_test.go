package gltf

import (
	"math"
	"testing"
)

func TestMaterialDefaultsAndSupportedProperties(t *testing.T) {
	parser := &parser{document: document{Materials: []materialDef{
		{
			DoubleSided: true,
			AlphaMode:   "MASK",
			AlphaCutoff: float64Pointer(1.5),
			PBR:         &pbrDef{BaseColorFactor: []float64{0.1, 0.2, 0.3, 0.4}},
		},
	}}}

	defaults, err := parser.material(nil)
	if err != nil {
		t.Fatalf("default material returned an error: %v", err)
	}
	if defaults != (Material{BaseColorFactor: [4]float32{1, 1, 1, 1}, BaseColorTexture: -1, AlphaMode: "OPAQUE", AlphaCutoff: 0.5}) {
		t.Fatalf("default material = %+v", defaults)
	}

	material, err := parser.material(intPointer(0))
	if err != nil {
		t.Fatalf("material returned an error: %v", err)
	}
	if !material.DoubleSided || material.AlphaMode != "MASK" || material.AlphaCutoff != 1.5 || material.BaseColorFactor != [4]float32{0.1, 0.2, 0.3, 0.4} {
		t.Fatalf("material = %+v", material)
	}
}

func TestMaterialRejectsInvalidProperties(t *testing.T) {
	tests := []struct {
		name       string
		definition materialDef
		index      int
		message    string
	}{
		{name: "index", index: 1, message: "out of range"},
		{name: "alpha mode", definition: materialDef{AlphaMode: "INVALID"}, message: "invalid alphaMode"},
		{name: "negative alpha cutoff", definition: materialDef{AlphaCutoff: float64Pointer(-0.1)}, message: "invalid alphaCutoff"},
		{name: "non-finite alpha cutoff", definition: materialDef{AlphaCutoff: float64Pointer(math.NaN())}, message: "invalid alphaCutoff"},
		{name: "factor length", definition: materialDef{PBR: &pbrDef{BaseColorFactor: []float64{1, 1, 1}}}, message: "four values"},
		{name: "negative factor", definition: materialDef{PBR: &pbrDef{BaseColorFactor: []float64{-0.1, 1, 1, 1}}}, message: "invalid value"},
		{name: "factor above one", definition: materialDef{PBR: &pbrDef{BaseColorFactor: []float64{1.1, 1, 1, 1}}}, message: "invalid value"},
		{name: "non-finite factor", definition: materialDef{PBR: &pbrDef{BaseColorFactor: []float64{math.NaN(), 1, 1, 1}}}, message: "invalid value"},
		{name: "unsupported texture coordinates", definition: materialDef{PBR: &pbrDef{BaseColorTexture: &textureInfoDef{Index: intPointer(0), TexCoord: 1}}}, message: "TEXCOORD_0"},
		{name: "missing texture index", definition: materialDef{PBR: &pbrDef{BaseColorTexture: &textureInfoDef{}}}, message: "index is required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := &parser{document: document{Materials: []materialDef{test.definition}}}
			_, err := parser.material(intPointer(test.index))
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestTextureResolutionUsesDefaultsCustomSamplerAndCache(t *testing.T) {
	parser := textureTestParser(document{
		Images: []imageDef{{URI: "data:image/png;base64,AQID"}},
		Textures: []textureDef{
			{Source: intPointer(0)},
			{Source: intPointer(0), Sampler: intPointer(0)},
		},
		Samplers: []samplerDef{{
			MagFilter: intPointer(9728), MinFilter: intPointer(9729),
			WrapS: intPointer(33071), WrapT: intPointer(33648),
		}},
	})

	first, err := parser.texture(0)
	if err != nil {
		t.Fatalf("default texture returned an error: %v", err)
	}
	again, err := parser.texture(0)
	if err != nil || again != first || len(parser.textures) != 1 {
		t.Fatalf("cached texture = %d, err = %v, texture count = %d", again, err, len(parser.textures))
	}
	second, err := parser.texture(1)
	if err != nil {
		t.Fatalf("custom texture returned an error: %v", err)
	}
	if second != 1 || len(parser.resolvedImages) != 1 {
		t.Fatalf("second texture = %d, image cache size = %d", second, len(parser.resolvedImages))
	}
	if parser.textures[0].Sampler != (Sampler{MagFilter: 9729, MinFilter: 9987, WrapS: 10497, WrapT: 10497}) {
		t.Fatalf("default sampler = %+v", parser.textures[0].Sampler)
	}
	if parser.textures[1].Sampler != (Sampler{MagFilter: 9728, MinFilter: 9729, WrapS: 33071, WrapT: 33648}) {
		t.Fatalf("custom sampler = %+v", parser.textures[1].Sampler)
	}
}

func TestTextureResolutionRejectsInvalidReferencesAndLimits(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*parser)
		index   int
		message string
	}{
		{name: "texture index", index: 1, message: "texture index"},
		{name: "missing source", index: 0, setup: func(p *parser) { p.document.Textures = []textureDef{{}} }, message: "invalid image source"},
		{name: "source out of range", index: 0, setup: func(p *parser) { p.document.Textures[0].Source = intPointer(1) }, message: "invalid image source"},
		{name: "sampler out of range", index: 0, setup: func(p *parser) { p.document.Textures[0].Sampler = intPointer(1) }, message: "invalid sampler"},
		{name: "invalid sampler value", index: 0, setup: func(p *parser) {
			p.document.Samplers = []samplerDef{{MagFilter: intPointer(1)}}
			p.document.Textures[0].Sampler = intPointer(0)
		}, message: "magnification filter"},
		{name: "per-image bytes", index: 0, setup: func(p *parser) { p.limits.MaxTextureBytes = 2 }, message: "texture byte limit"},
		{name: "total bytes", index: 0, setup: func(p *parser) { p.limits.MaxTextureBytesTotal = 2 }, message: "total texture byte limit"},
		{name: "texture count", index: 0, setup: func(p *parser) { p.limits.MaxTextures = 1; p.textures = append(p.textures, Texture{}) }, message: "texture count limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := textureTestParser(document{
				Images:   []imageDef{{URI: "data:image/png;base64,AQID"}},
				Textures: []textureDef{{Source: intPointer(0)}},
			})
			if test.setup != nil {
				test.setup(parser)
			}
			_, err := parser.texture(test.index)
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestImageReadsDataURIsAndCopiesBufferViews(t *testing.T) {
	t.Run("base64", func(t *testing.T) {
		parser := textureTestParser(document{})
		data, mimeType, err := parser.image(imageDef{URI: "data:image/png;base64,AQID"})
		if err != nil || mimeType != "image/png" || string(data) != string([]byte{1, 2, 3}) {
			t.Fatalf("image = %v, %q, err = %v", data, mimeType, err)
		}
	})
	t.Run("percent encoded", func(t *testing.T) {
		parser := textureTestParser(document{})
		data, mimeType, err := parser.image(imageDef{URI: "data:image/webp,%01%02%03"})
		if err != nil || mimeType != "image/webp" || string(data) != string([]byte{1, 2, 3}) {
			t.Fatalf("image = %v, %q, err = %v", data, mimeType, err)
		}
	})
	t.Run("buffer view copy", func(t *testing.T) {
		parser := textureTestParser(document{BufferViews: []bufferViewDef{{Buffer: 0, ByteLength: 3}}})
		parser.bin = []byte{1, 2, 3}
		data, mimeType, err := parser.image(imageDef{BufferView: intPointer(0), MIMEType: "image/jpeg"})
		if err != nil || mimeType != "image/jpeg" || string(data) != string([]byte{1, 2, 3}) {
			t.Fatalf("image = %v, %q, err = %v", data, mimeType, err)
		}
		parser.bin[0] = 9
		if data[0] != 1 {
			t.Fatal("buffer-view image retained the full GLB backing allocation")
		}
	})
}

func TestImageRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name       string
		definition imageDef
		message    string
	}{
		{name: "external URI", definition: imageDef{URI: "texture.png"}, message: "external image URIs"},
		{name: "missing comma", definition: imageDef{URI: "data:image/png;base64"}, message: "invalid image data URI"},
		{name: "bad base64", definition: imageDef{URI: "data:image/png;base64,!"}, message: "decode image data URI"},
		{name: "unsupported MIME", definition: imageDef{URI: "data:image/gif;base64,AQ=="}, message: "unsupported image MIME"},
		{name: "empty data", definition: imageDef{URI: "data:image/png;base64,"}, message: "image data is empty"},
		{name: "URI and view", definition: imageDef{URI: "data:image/png;base64,AQ==", BufferView: intPointer(0)}, message: "both URI and bufferView"},
		{name: "no source", definition: imageDef{}, message: "neither a URI nor a bufferView"},
		{name: "view out of range", definition: imageDef{BufferView: intPointer(0), MIMEType: "image/png"}, message: "out of range"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := textureTestParser(document{})
			_, _, err := parser.image(test.definition)
			assertErrorContains(t, err, test.message)
		})
	}
}

func TestValidateSamplerAcceptsOnlyWebGLSamplerEnums(t *testing.T) {
	valid := []Sampler{
		{MagFilter: 9728, MinFilter: 9728, WrapS: 33071, WrapT: 33071},
		{MagFilter: 9729, MinFilter: 9984, WrapS: 33648, WrapT: 33648},
		{MagFilter: 9729, MinFilter: 9987, WrapS: 10497, WrapT: 10497},
	}
	for _, sampler := range valid {
		if err := validateSampler(sampler); err != nil {
			t.Errorf("valid sampler %+v rejected: %v", sampler, err)
		}
	}
	invalid := []struct {
		sampler Sampler
		message string
	}{
		{sampler: Sampler{MagFilter: 1, MinFilter: 9728, WrapS: 10497, WrapT: 10497}, message: "magnification"},
		{sampler: Sampler{MagFilter: 9728, MinFilter: 1, WrapS: 10497, WrapT: 10497}, message: "minification"},
		{sampler: Sampler{MagFilter: 9728, MinFilter: 9728, WrapS: 1, WrapT: 10497}, message: "wrap mode"},
	}
	for _, test := range invalid {
		assertErrorContains(t, validateSampler(test.sampler), test.message)
	}
}

func TestBufferViewBytesValidation(t *testing.T) {
	tests := []struct {
		name    string
		index   int
		view    bufferViewDef
		message string
	}{
		{name: "negative index", index: -1, message: "out of range"},
		{name: "high index", index: 1, message: "out of range"},
		{name: "secondary buffer", view: bufferViewDef{Buffer: 1, ByteLength: 1}, message: "is invalid"},
		{name: "negative offset", view: bufferViewDef{ByteOffset: -1, ByteLength: 1}, message: "is invalid"},
		{name: "negative length", view: bufferViewDef{ByteLength: -1}, message: "is invalid"},
		{name: "past BIN", view: bufferViewDef{ByteOffset: 3, ByteLength: 2}, message: "exceeds the BIN"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parser := &parser{document: document{BufferViews: []bufferViewDef{test.view}}, bin: make([]byte, 4)}
			_, err := parser.bufferViewBytes(test.index)
			assertErrorContains(t, err, test.message)
		})
	}
	parser := &parser{document: document{BufferViews: []bufferViewDef{{ByteOffset: 1, ByteLength: 2}}}, bin: []byte{0, 1, 2, 3}}
	data, err := parser.bufferViewBytes(0)
	if err != nil || string(data) != string([]byte{1, 2}) {
		t.Fatalf("bufferViewBytes = %v, err = %v", data, err)
	}
}

func textureTestParser(definition document) *parser {
	return &parser{
		document:         definition,
		limits:           DefaultLimits(),
		resolvedTextures: make(map[int]int),
		resolvedImages:   make(map[int]resolvedImage),
	}
}

func float64Pointer(value float64) *float64 {
	return &value
}
