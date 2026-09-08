package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"strings"
)

const (
	glbMagic     = 0x46546c67
	jsonChunk    = 0x4e4f534a
	binChunk     = 0x004e4942
	glbHeaderLen = 12
)

type parser struct {
	document          document
	bin               []byte
	limits            Limits
	reader            accessorReader
	parsedMeshes      [][]Primitive
	textures          []Texture
	resolvedTextures  map[int]int
	resolvedImages    map[int]resolvedImage
	parsedVertices    int
	parsedIndices     int
	instancedVertices int
	instancedIndices  int
	textureBytesTotal int
}

type resolvedImage struct {
	data     []byte
	mimeType string
}

func ParseGLB(data []byte) (*Model, error) {
	return ParseGLBWithLimits(data, DefaultLimits())
}

func ParseGLBWithLimits(data []byte, limits Limits) (*Model, error) {
	if err := validateLimits(limits); err != nil {
		return nil, err
	}
	jsonData, bin, err := parseContainer(data, limits)
	if err != nil {
		return nil, err
	}

	var definition document
	if err := json.Unmarshal(bytes.TrimRight(jsonData, " \t\r\n\x00"), &definition); err != nil {
		return nil, fmt.Errorf("decode GLTF JSON: %w", err)
	}
	if definition.Asset.Version != "2.0" {
		return nil, fmt.Errorf("GLTF asset.version must be 2.0")
	}
	if len(definition.ExtensionsRequired) > 0 {
		return nil, fmt.Errorf("required GLTF extensions are unsupported: %s", strings.Join(definition.ExtensionsRequired, ", "))
	}
	if len(definition.Skins) > 0 {
		return nil, fmt.Errorf("skinned models are not supported")
	}
	if len(definition.Nodes) > limits.MaxNodes {
		return nil, fmt.Errorf("model contains too many nodes")
	}
	if definition.Scene != nil && (*definition.Scene < 0 || *definition.Scene >= len(definition.Scenes)) {
		return nil, fmt.Errorf("selected scene index %d is out of range", *definition.Scene)
	}
	if err := validateNodeGraph(&definition, limits); err != nil {
		return nil, err
	}
	if len(definition.Buffers) > 1 {
		return nil, fmt.Errorf("GLB files with multiple buffers are not supported")
	}
	if len(definition.BufferViews) > 0 && len(definition.Buffers) != 1 {
		return nil, fmt.Errorf("GLB files with bufferViews must declare exactly one embedded buffer")
	}
	if len(definition.Buffers) == 0 && bin != nil {
		return nil, fmt.Errorf("GLB has a BIN chunk but does not declare an embedded buffer")
	}
	if len(definition.Buffers) > 0 {
		buffer := definition.Buffers[0]
		if buffer.URI != "" {
			return nil, fmt.Errorf("the primary GLB buffer must use the BIN chunk")
		}
		if buffer.ByteLength <= 0 || buffer.ByteLength > len(bin) {
			return nil, fmt.Errorf("declared buffer length is invalid for the BIN chunk")
		}
		if len(bin)-buffer.ByteLength > 3 {
			return nil, fmt.Errorf("BIN chunk contains data outside the declared buffer")
		}
		bin = bin[:buffer.ByteLength]
	}

	p := &parser{
		document:         definition,
		bin:              bin,
		limits:           limits,
		resolvedTextures: make(map[int]int),
		resolvedImages:   make(map[int]resolvedImage),
	}
	p.reader = accessorReader{document: &p.document, bin: bin, limits: limits}
	if err := p.parseMeshes(); err != nil {
		return nil, err
	}
	model, err := p.instantiateScene()
	if err != nil {
		return nil, err
	}
	model.Textures = p.textures
	if len(model.Primitives) == 0 {
		return nil, fmt.Errorf("selected scene contains no renderable primitives")
	}
	return model, nil
}

func validateLimits(limits Limits) error {
	values := []int{
		limits.MaxFileBytes,
		limits.MaxJSONBytes,
		limits.MaxTextureBytes,
		limits.MaxTextureBytesTotal,
		limits.MaxTextures,
		limits.MaxAccessorCount,
		limits.MaxVertices,
		limits.MaxIndices,
		limits.MaxPrimitives,
		limits.MaxNodes,
		limits.MaxNodeDepth,
	}
	for _, value := range values {
		if value <= 0 {
			return fmt.Errorf("all parser limits must be positive")
		}
	}
	return nil
}

func validateNodeGraph(definition *document, limits Limits) error {
	parents := make([]int, len(definition.Nodes))
	for index := range parents {
		parents[index] = -1
	}
	for nodeIndex, node := range definition.Nodes {
		if node.Skin != nil {
			return fmt.Errorf("node %d uses an unsupported skin", nodeIndex)
		}
		if node.Mesh != nil && (*node.Mesh < 0 || *node.Mesh >= len(definition.Meshes)) {
			return fmt.Errorf("node %d has an invalid mesh index %d", nodeIndex, *node.Mesh)
		}
		if _, err := nodeMatrix(node); err != nil {
			return fmt.Errorf("node %d: %w", nodeIndex, err)
		}
		for _, child := range node.Children {
			if child < 0 || child >= len(definition.Nodes) {
				return fmt.Errorf("node %d has an invalid child index %d", nodeIndex, child)
			}
			if parents[child] != -1 {
				return fmt.Errorf("node %d is the child of more than one node", child)
			}
			parents[child] = nodeIndex
		}
	}
	for sceneIndex, scene := range definition.Scenes {
		seen := make(map[int]struct{}, len(scene.Nodes))
		for _, root := range scene.Nodes {
			if root < 0 || root >= len(definition.Nodes) {
				return fmt.Errorf("scene %d has an invalid root node index %d", sceneIndex, root)
			}
			if _, exists := seen[root]; exists {
				return fmt.Errorf("scene %d references root node %d more than once", sceneIndex, root)
			}
			seen[root] = struct{}{}
		}
	}

	state := make([]uint8, len(definition.Nodes))
	var visit func(int, int) error
	visit = func(index, depth int) error {
		if depth >= limits.MaxNodeDepth {
			return fmt.Errorf("node hierarchy exceeds the depth limit")
		}
		if state[index] == 1 {
			return fmt.Errorf("node hierarchy contains a cycle at node %d", index)
		}
		if state[index] == 2 {
			return nil
		}
		state[index] = 1
		for _, child := range definition.Nodes[index].Children {
			if err := visit(child, depth+1); err != nil {
				return err
			}
		}
		state[index] = 2
		return nil
	}
	for index, parent := range parents {
		if parent == -1 {
			if err := visit(index, 0); err != nil {
				return err
			}
		}
	}
	for index := range definition.Nodes {
		if state[index] == 0 {
			if err := visit(index, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseContainer(data []byte, limits Limits) ([]byte, []byte, error) {
	if len(data) < glbHeaderLen {
		return nil, nil, fmt.Errorf("GLB header is truncated")
	}
	if len(data) > limits.MaxFileBytes {
		return nil, nil, fmt.Errorf("GLB exceeds the %d-byte file limit", limits.MaxFileBytes)
	}
	if binary.LittleEndian.Uint32(data[0:4]) != glbMagic {
		return nil, nil, fmt.Errorf("invalid GLB magic")
	}
	if binary.LittleEndian.Uint32(data[4:8]) != 2 {
		return nil, nil, fmt.Errorf("only GLB version 2 is supported")
	}
	declaredLength := uint64(binary.LittleEndian.Uint32(data[8:12]))
	if declaredLength != uint64(len(data)) {
		return nil, nil, fmt.Errorf("GLB declared length %d does not match actual length %d", declaredLength, len(data))
	}

	offset := glbHeaderLen
	chunkNumber := 0
	var jsonData, bin []byte
	for offset < len(data) {
		if len(data)-offset < 8 {
			return nil, nil, fmt.Errorf("GLB chunk header is truncated")
		}
		chunkLength := uint64(binary.LittleEndian.Uint32(data[offset : offset+4]))
		chunkType := binary.LittleEndian.Uint32(data[offset+4 : offset+8])
		offset += 8
		if chunkLength%4 != 0 {
			return nil, nil, fmt.Errorf("GLB chunk length is not four-byte aligned")
		}
		if chunkLength > uint64(len(data)-offset) {
			return nil, nil, fmt.Errorf("GLB chunk exceeds the declared file length")
		}
		end := offset + int(chunkLength)
		chunk := data[offset:end]
		if chunkNumber == 0 && chunkType != jsonChunk {
			return nil, nil, fmt.Errorf("the first GLB chunk must contain JSON")
		}
		switch chunkType {
		case jsonChunk:
			if jsonData != nil {
				return nil, nil, fmt.Errorf("GLB contains multiple JSON chunks")
			}
			if len(chunk) > limits.MaxJSONBytes {
				return nil, nil, fmt.Errorf("GLTF JSON exceeds the configured limit")
			}
			jsonData = chunk
		case binChunk:
			if bin != nil {
				return nil, nil, fmt.Errorf("GLB contains multiple BIN chunks")
			}
			bin = chunk
		}
		offset = end
		chunkNumber++
	}
	if jsonData == nil {
		return nil, nil, fmt.Errorf("GLB has no JSON chunk")
	}
	return jsonData, bin, nil
}

func (p *parser) parseMeshes() error {
	p.parsedMeshes = make([][]Primitive, len(p.document.Meshes))
	primitiveCount := 0
	for meshIndex, mesh := range p.document.Meshes {
		for primitiveIndex, definition := range mesh.Primitives {
			primitiveCount++
			if primitiveCount > p.limits.MaxPrimitives {
				return fmt.Errorf("model contains too many primitives")
			}
			primitive, err := p.parsePrimitive(definition)
			if err != nil {
				return fmt.Errorf("mesh %d primitive %d: %w", meshIndex, primitiveIndex, err)
			}
			vertexCount := len(primitive.Positions) / 3
			if vertexCount > p.limits.MaxVertices-p.parsedVertices {
				return fmt.Errorf("model exceeds the total vertex limit")
			}
			if len(primitive.Indices) > p.limits.MaxIndices-p.parsedIndices {
				return fmt.Errorf("model exceeds the total index limit")
			}
			p.parsedVertices += vertexCount
			p.parsedIndices += len(primitive.Indices)
			p.parsedMeshes[meshIndex] = append(p.parsedMeshes[meshIndex], primitive)
		}
	}
	return nil
}

func (p *parser) parsePrimitive(definition primitiveDef) (Primitive, error) {
	if len(definition.Targets) > 0 {
		return Primitive{}, fmt.Errorf("morph targets are not supported")
	}
	positionIndex, ok := definition.Attributes["POSITION"]
	if !ok {
		return Primitive{}, fmt.Errorf("POSITION attribute is required")
	}
	positions, vertexCount, err := p.reader.positions(positionIndex)
	if err != nil {
		return Primitive{}, err
	}

	var indices []uint32
	if definition.Indices == nil {
		indices = make([]uint32, vertexCount)
		for i := range indices {
			indices[i] = uint32(i)
		}
	} else {
		indices, err = p.reader.indices(*definition.Indices, vertexCount)
		if err != nil {
			return Primitive{}, err
		}
	}

	mode := uint32(4)
	if definition.Mode != nil {
		mode = *definition.Mode
	}
	if err := validateTopology(mode, len(indices)); err != nil {
		return Primitive{}, err
	}

	var normals []float32
	if normalIndex, exists := definition.Attributes["NORMAL"]; exists {
		normals, err = p.reader.normals(normalIndex, vertexCount)
		if err != nil {
			return Primitive{}, err
		}
	} else {
		normals = generateNormals(positions, indices, mode)
	}

	uvs := make([]float32, vertexCount*2)
	uvIndex, hasUVs := definition.Attributes["TEXCOORD_0"]
	if hasUVs {
		uvs, err = p.reader.textureCoordinates(uvIndex, vertexCount)
		if err != nil {
			return Primitive{}, err
		}
	}
	material, err := p.material(definition.Material)
	if err != nil {
		return Primitive{}, err
	}
	if material.BaseColorTexture >= 0 && !hasUVs {
		return Primitive{}, fmt.Errorf("base-color texture requires TEXCOORD_0")
	}
	return Primitive{
		Positions: positions,
		Normals:   normals,
		UVs:       uvs,
		Indices:   indices,
		Mode:      mode,
		Transform: Identity(),
		Material:  material,
	}, nil
}

func validateTopology(mode uint32, indexCount int) error {
	switch mode {
	case 0:
		return nil
	case 1:
		if indexCount%2 != 0 {
			return fmt.Errorf("LINES primitive must contain an even number of indices")
		}
	case 2, 3:
		if indexCount < 2 {
			return fmt.Errorf("line strip/loop primitive requires at least two indices")
		}
	case 4:
		if indexCount%3 != 0 {
			return fmt.Errorf("TRIANGLES primitive index count must be divisible by three")
		}
	case 5, 6:
		if indexCount < 3 {
			return fmt.Errorf("triangle strip/fan primitive requires at least three indices")
		}
	default:
		return fmt.Errorf("unsupported primitive mode %d", mode)
	}
	return nil
}

func generateNormals(positions []float32, indices []uint32, mode uint32) []float32 {
	accumulated := make([]float64, len(positions))
	addTriangle := func(a, b, c uint32) {
		ai, bi, ci := int(a)*3, int(b)*3, int(c)*3
		ax, ay, az := float64(positions[ai]), float64(positions[ai+1]), float64(positions[ai+2])
		bx, by, bz := float64(positions[bi]), float64(positions[bi+1]), float64(positions[bi+2])
		cx, cy, cz := float64(positions[ci]), float64(positions[ci+1]), float64(positions[ci+2])
		abx, aby, abz := bx-ax, by-ay, bz-az
		acx, acy, acz := cx-ax, cy-ay, cz-az
		nx, ny, nz := aby*acz-abz*acy, abz*acx-abx*acz, abx*acy-aby*acx
		for _, index := range []int{ai, bi, ci} {
			accumulated[index] += nx
			accumulated[index+1] += ny
			accumulated[index+2] += nz
		}
	}
	switch mode {
	case 4:
		for i := 0; i+2 < len(indices); i += 3 {
			addTriangle(indices[i], indices[i+1], indices[i+2])
		}
	case 5:
		for i := 0; i+2 < len(indices); i++ {
			if i%2 == 0 {
				addTriangle(indices[i], indices[i+1], indices[i+2])
			} else {
				addTriangle(indices[i+1], indices[i], indices[i+2])
			}
		}
	case 6:
		for i := 1; i+1 < len(indices); i++ {
			addTriangle(indices[0], indices[i], indices[i+1])
		}
	}
	normals := make([]float32, len(positions))
	for i := 0; i < len(accumulated); i += 3 {
		length := math.Sqrt(accumulated[i]*accumulated[i] + accumulated[i+1]*accumulated[i+1] + accumulated[i+2]*accumulated[i+2])
		if length == 0 {
			normals[i+2] = 1
			continue
		}
		normals[i] = float32(accumulated[i] / length)
		normals[i+1] = float32(accumulated[i+1] / length)
		normals[i+2] = float32(accumulated[i+2] / length)
	}
	return normals
}

func (p *parser) material(index *int) (Material, error) {
	result := Material{
		BaseColorFactor:  [4]float32{1, 1, 1, 1},
		BaseColorTexture: -1,
		AlphaMode:        "OPAQUE",
		AlphaCutoff:      0.5,
	}
	if index == nil {
		return result, nil
	}
	if *index < 0 || *index >= len(p.document.Materials) {
		return Material{}, fmt.Errorf("material index %d is out of range", *index)
	}
	definition := p.document.Materials[*index]
	result.DoubleSided = definition.DoubleSided
	if definition.AlphaMode != "" {
		if definition.AlphaMode != "OPAQUE" && definition.AlphaMode != "MASK" && definition.AlphaMode != "BLEND" {
			return Material{}, fmt.Errorf("material has invalid alphaMode %q", definition.AlphaMode)
		}
		result.AlphaMode = definition.AlphaMode
	}
	if definition.AlphaCutoff != nil {
		if !finite(*definition.AlphaCutoff) || *definition.AlphaCutoff < 0 {
			return Material{}, fmt.Errorf("material has invalid alphaCutoff")
		}
		result.AlphaCutoff = float32(*definition.AlphaCutoff)
	}
	if definition.PBR == nil {
		return result, nil
	}
	if len(definition.PBR.BaseColorFactor) != 0 {
		if len(definition.PBR.BaseColorFactor) != 4 {
			return Material{}, fmt.Errorf("baseColorFactor must contain four values")
		}
		for i, value := range definition.PBR.BaseColorFactor {
			if !finite(value) || value < 0 || value > 1 {
				return Material{}, fmt.Errorf("baseColorFactor contains an invalid value")
			}
			result.BaseColorFactor[i] = float32(value)
		}
	}
	if definition.PBR.BaseColorTexture != nil {
		if definition.PBR.BaseColorTexture.TexCoord != 0 {
			return Material{}, fmt.Errorf("only TEXCOORD_0 base-color textures are supported")
		}
		if definition.PBR.BaseColorTexture.Index == nil {
			return Material{}, fmt.Errorf("baseColorTexture index is required")
		}
		resolved, err := p.texture(*definition.PBR.BaseColorTexture.Index)
		if err != nil {
			return Material{}, err
		}
		result.BaseColorTexture = resolved
	}
	return result, nil
}

func (p *parser) texture(index int) (int, error) {
	if resolved, exists := p.resolvedTextures[index]; exists {
		return resolved, nil
	}
	if index < 0 || index >= len(p.document.Textures) {
		return -1, fmt.Errorf("texture index %d is out of range", index)
	}
	definition := p.document.Textures[index]
	if definition.Source == nil || *definition.Source < 0 || *definition.Source >= len(p.document.Images) {
		return -1, fmt.Errorf("texture %d has an invalid image source", index)
	}
	if len(p.textures) >= p.limits.MaxTextures {
		return -1, fmt.Errorf("model exceeds the texture count limit")
	}
	data, mimeType, err := p.cachedImage(*definition.Source)
	if err != nil {
		return -1, fmt.Errorf("texture %d: %w", index, err)
	}
	sampler := Sampler{MagFilter: 9729, MinFilter: 9987, WrapS: 10497, WrapT: 10497}
	if definition.Sampler != nil {
		if *definition.Sampler < 0 || *definition.Sampler >= len(p.document.Samplers) {
			return -1, fmt.Errorf("texture %d has an invalid sampler", index)
		}
		source := p.document.Samplers[*definition.Sampler]
		if source.MagFilter != nil {
			sampler.MagFilter = *source.MagFilter
		}
		if source.MinFilter != nil {
			sampler.MinFilter = *source.MinFilter
		}
		if source.WrapS != nil {
			sampler.WrapS = *source.WrapS
		}
		if source.WrapT != nil {
			sampler.WrapT = *source.WrapT
		}
	}
	if err := validateSampler(sampler); err != nil {
		return -1, fmt.Errorf("texture %d: %w", index, err)
	}
	if len(data) > p.limits.MaxTextureBytesTotal-p.textureBytesTotal {
		return -1, fmt.Errorf("model exceeds the total texture byte limit")
	}
	p.textureBytesTotal += len(data)
	resolved := len(p.textures)
	p.textures = append(p.textures, Texture{Data: data, MIMEType: mimeType, Sampler: sampler})
	p.resolvedTextures[index] = resolved
	return resolved, nil
}

func (p *parser) cachedImage(index int) ([]byte, string, error) {
	if image, exists := p.resolvedImages[index]; exists {
		return image.data, image.mimeType, nil
	}
	data, mimeType, err := p.image(p.document.Images[index])
	if err != nil {
		return nil, "", err
	}
	p.resolvedImages[index] = resolvedImage{data: data, mimeType: mimeType}
	return data, mimeType, nil
}

func validateSampler(sampler Sampler) error {
	if sampler.MagFilter != 9728 && sampler.MagFilter != 9729 {
		return fmt.Errorf("invalid magnification filter %d", sampler.MagFilter)
	}
	validMin := sampler.MinFilter == 9728 || sampler.MinFilter == 9729 || (sampler.MinFilter >= 9984 && sampler.MinFilter <= 9987)
	if !validMin {
		return fmt.Errorf("invalid minification filter %d", sampler.MinFilter)
	}
	for _, wrap := range []int{sampler.WrapS, sampler.WrapT} {
		if wrap != 33071 && wrap != 33648 && wrap != 10497 {
			return fmt.Errorf("invalid wrap mode %d", wrap)
		}
	}
	return nil
}

func (p *parser) image(definition imageDef) ([]byte, string, error) {
	if definition.URI != "" {
		if definition.BufferView != nil {
			return nil, "", fmt.Errorf("image cannot define both URI and bufferView")
		}
		if !strings.HasPrefix(definition.URI, "data:") {
			return nil, "", fmt.Errorf("external image URIs are not supported for uploaded GLB files")
		}
		comma := strings.IndexByte(definition.URI, ',')
		if comma < 0 {
			return nil, "", fmt.Errorf("invalid image data URI")
		}
		metadata, payload := definition.URI[5:comma], definition.URI[comma+1:]
		parts := strings.Split(metadata, ";")
		mimeType := parts[0]
		var data []byte
		var err error
		if slicesContain(parts[1:], "base64") {
			data, err = base64.StdEncoding.DecodeString(payload)
		} else {
			var decoded string
			decoded, err = url.PathUnescape(payload)
			data = []byte(decoded)
		}
		if err != nil {
			return nil, "", fmt.Errorf("decode image data URI: %w", err)
		}
		if err := validateImage(mimeType, data); err != nil {
			return nil, "", err
		}
		if len(data) > p.limits.MaxTextureBytes {
			return nil, "", fmt.Errorf("image exceeds the configured texture byte limit")
		}
		return data, mimeType, nil
	}
	if definition.BufferView == nil {
		return nil, "", fmt.Errorf("image has neither a URI nor a bufferView")
	}
	data, err := p.bufferViewBytes(*definition.BufferView)
	if err != nil {
		return nil, "", err
	}
	if err := validateImage(definition.MIMEType, data); err != nil {
		return nil, "", err
	}
	if len(data) > p.limits.MaxTextureBytes {
		return nil, "", fmt.Errorf("image exceeds the configured texture byte limit")
	}
	return append([]byte(nil), data...), definition.MIMEType, nil
}

func validateImage(mimeType string, data []byte) error {
	if mimeType != "image/png" && mimeType != "image/jpeg" && mimeType != "image/webp" {
		return fmt.Errorf("unsupported image MIME type %q", mimeType)
	}
	if len(data) == 0 {
		return fmt.Errorf("image data is empty")
	}
	return nil
}

func slicesContain(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func (p *parser) bufferViewBytes(index int) ([]byte, error) {
	if index < 0 || index >= len(p.document.BufferViews) {
		return nil, fmt.Errorf("bufferView index %d is out of range", index)
	}
	view := p.document.BufferViews[index]
	if view.Buffer != 0 || view.ByteOffset < 0 || view.ByteLength < 0 {
		return nil, fmt.Errorf("bufferView %d is invalid", index)
	}
	end, ok := checkedAdd(view.ByteOffset, view.ByteLength)
	if !ok || end > len(p.bin) {
		return nil, fmt.Errorf("bufferView %d exceeds the BIN chunk", index)
	}
	return p.bin[view.ByteOffset:end], nil
}

func (p *parser) instantiateScene() (*Model, error) {
	model := &Model{}
	if len(p.document.Nodes) == 0 && len(p.document.Scenes) == 0 {
		for meshIndex := range p.parsedMeshes {
			if err := p.appendMesh(model, meshIndex, Identity()); err != nil {
				return nil, err
			}
		}
		return model, nil
	}

	var roots []int
	if len(p.document.Scenes) > 0 {
		sceneIndex := 0
		if p.document.Scene != nil {
			sceneIndex = *p.document.Scene
		}
		if sceneIndex < 0 || sceneIndex >= len(p.document.Scenes) {
			return nil, fmt.Errorf("selected scene index %d is out of range", sceneIndex)
		}
		roots = p.document.Scenes[sceneIndex].Nodes
	} else {
		isChild := make([]bool, len(p.document.Nodes))
		for _, node := range p.document.Nodes {
			for _, child := range node.Children {
				if child < 0 || child >= len(isChild) {
					return nil, fmt.Errorf("node child index %d is out of range", child)
				}
				isChild[child] = true
			}
		}
		for index, child := range isChild {
			if !child {
				roots = append(roots, index)
			}
		}
	}
	stack := make([]bool, len(p.document.Nodes))
	visited := make([]bool, len(p.document.Nodes))
	for _, root := range roots {
		if err := p.walkNode(model, root, Identity(), stack, visited, 0); err != nil {
			return nil, err
		}
	}
	return model, nil
}

func (p *parser) walkNode(model *Model, index int, parent Mat4, stack, visited []bool, depth int) error {
	if index < 0 || index >= len(p.document.Nodes) {
		return fmt.Errorf("node index %d is out of range", index)
	}
	if depth >= p.limits.MaxNodeDepth {
		return fmt.Errorf("node hierarchy exceeds the depth limit")
	}
	if stack[index] {
		return fmt.Errorf("node hierarchy contains a cycle at node %d", index)
	}
	if visited[index] {
		return fmt.Errorf("node %d is referenced more than once in the selected scene", index)
	}
	stack[index] = true
	visited[index] = true
	defer func() { stack[index] = false }()
	node := p.document.Nodes[index]
	if node.Skin != nil {
		return fmt.Errorf("node %d uses an unsupported skin", index)
	}
	local, err := nodeMatrix(node)
	if err != nil {
		return fmt.Errorf("node %d: %w", index, err)
	}
	world := multiply(parent, local)
	if node.Mesh != nil {
		if err := p.appendMesh(model, *node.Mesh, world); err != nil {
			return fmt.Errorf("node %d: %w", index, err)
		}
	}
	for _, child := range node.Children {
		if err := p.walkNode(model, child, world, stack, visited, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (p *parser) appendMesh(model *Model, index int, transform Mat4) error {
	if index < 0 || index >= len(p.parsedMeshes) {
		return fmt.Errorf("mesh index %d is out of range", index)
	}
	for _, source := range p.parsedMeshes[index] {
		if len(model.Primitives) >= p.limits.MaxPrimitives {
			return fmt.Errorf("instantiated scene contains too many primitives")
		}
		vertexCount := len(source.Positions) / 3
		if vertexCount > p.limits.MaxVertices-p.instancedVertices {
			return fmt.Errorf("selected scene exceeds the total vertex limit")
		}
		if len(source.Indices) > p.limits.MaxIndices-p.instancedIndices {
			return fmt.Errorf("selected scene exceeds the total index limit")
		}
		p.instancedVertices += vertexCount
		p.instancedIndices += len(source.Indices)
		primitive := source
		primitive.Transform = transform
		model.Primitives = append(model.Primitives, primitive)
		for vertex := 0; vertex < len(primitive.Positions); vertex += 3 {
			point, err := transformPoint(transform, primitive.Positions[vertex], primitive.Positions[vertex+1], primitive.Positions[vertex+2])
			if err != nil {
				return err
			}
			expandBounds(&model.Bounds, point)
		}
	}
	return nil
}

func expandBounds(bounds *Bounds, point [3]float32) {
	if !bounds.Valid {
		bounds.Min, bounds.Max, bounds.Valid = point, point, true
		return
	}
	for axis := range point {
		bounds.Min[axis] = min(bounds.Min[axis], point[axis])
		bounds.Max[axis] = max(bounds.Max[axis], point[axis])
	}
}
