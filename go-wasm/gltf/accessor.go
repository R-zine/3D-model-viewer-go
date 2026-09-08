package gltf

import (
	"encoding/binary"
	"fmt"
	"math"
)

type accessorReader struct {
	document *document
	bin      []byte
	limits   Limits
}

func (reader accessorReader) positions(index int) ([]float32, int, error) {
	accessor, err := reader.accessor(index, "VEC3")
	if err != nil {
		return nil, 0, err
	}
	if accessor.ComponentType != 5126 || accessor.Normalized {
		return nil, 0, fmt.Errorf("POSITION accessor %d must use non-normalized FLOAT components", index)
	}
	if accessor.Count > reader.limits.MaxVertices {
		return nil, 0, fmt.Errorf("POSITION accessor %d exceeds the vertex limit", index)
	}
	values, err := reader.floats(accessor)
	return values, accessor.Count, err
}

func (reader accessorReader) normals(index, vertexCount int) ([]float32, error) {
	accessor, err := reader.accessor(index, "VEC3")
	if err != nil {
		return nil, err
	}
	if accessor.Count != vertexCount {
		return nil, fmt.Errorf("NORMAL accessor %d count does not match POSITION", index)
	}
	valid := accessor.ComponentType == 5126 && !accessor.Normalized
	valid = valid || ((accessor.ComponentType == 5120 || accessor.ComponentType == 5122) && accessor.Normalized)
	if !valid {
		return nil, fmt.Errorf("NORMAL accessor %d has an unsupported component encoding", index)
	}
	return reader.floats(accessor)
}

func (reader accessorReader) textureCoordinates(index, vertexCount int) ([]float32, error) {
	accessor, err := reader.accessor(index, "VEC2")
	if err != nil {
		return nil, err
	}
	if accessor.Count != vertexCount {
		return nil, fmt.Errorf("TEXCOORD_0 accessor %d count does not match POSITION", index)
	}
	valid := accessor.ComponentType == 5126 && !accessor.Normalized
	valid = valid || ((accessor.ComponentType == 5121 || accessor.ComponentType == 5123) && accessor.Normalized)
	if !valid {
		return nil, fmt.Errorf("TEXCOORD_0 accessor %d has an unsupported component encoding", index)
	}
	return reader.floats(accessor)
}

func (reader accessorReader) indices(index, vertexCount int) ([]uint32, error) {
	accessor, err := reader.accessor(index, "SCALAR")
	if err != nil {
		return nil, err
	}
	if accessor.Normalized || (accessor.ComponentType != 5121 && accessor.ComponentType != 5123 && accessor.ComponentType != 5125) {
		return nil, fmt.Errorf("index accessor %d must use an unsigned integer component type", index)
	}
	if accessor.Count > reader.limits.MaxIndices {
		return nil, fmt.Errorf("index accessor %d exceeds the index limit", index)
	}

	_, start, stride, _, err := reader.layout(accessor)
	if err != nil {
		return nil, err
	}
	componentSize, _ := componentSize(accessor.ComponentType)
	result := make([]uint32, accessor.Count)
	for i := range result {
		offset := start + i*stride
		switch accessor.ComponentType {
		case 5121:
			result[i] = uint32(reader.bin[offset])
		case 5123:
			result[i] = uint32(binary.LittleEndian.Uint16(reader.bin[offset : offset+componentSize]))
		case 5125:
			result[i] = binary.LittleEndian.Uint32(reader.bin[offset : offset+componentSize])
		}
		if result[i] >= uint32(vertexCount) {
			return nil, fmt.Errorf("index accessor references vertex %d but only %d vertices exist", result[i], vertexCount)
		}
	}
	return result, nil
}

func (reader accessorReader) accessor(index int, expectedType string) (*accessorDef, error) {
	if index < 0 || index >= len(reader.document.Accessors) {
		return nil, fmt.Errorf("accessor index %d is out of range", index)
	}
	accessor := &reader.document.Accessors[index]
	if accessor.Type != expectedType {
		return nil, fmt.Errorf("accessor %d must have type %s, got %s", index, expectedType, accessor.Type)
	}
	if accessor.Count <= 0 || accessor.Count > reader.limits.MaxAccessorCount {
		return nil, fmt.Errorf("accessor %d has invalid count %d", index, accessor.Count)
	}
	if accessor.ByteOffset < 0 {
		return nil, fmt.Errorf("accessor %d has a negative byte offset", index)
	}
	if accessor.Sparse != nil {
		return nil, fmt.Errorf("sparse accessor %d is not supported", index)
	}
	if accessor.BufferView == nil {
		return nil, fmt.Errorf("accessor %d has no bufferView", index)
	}
	return accessor, nil
}

func (reader accessorReader) floats(accessor *accessorDef) ([]float32, error) {
	_, start, stride, componentCount, err := reader.layout(accessor)
	if err != nil {
		return nil, err
	}
	componentSize, _ := componentSize(accessor.ComponentType)
	valueCount, ok := checkedMultiply(accessor.Count, componentCount)
	if !ok {
		return nil, fmt.Errorf("accessor value count overflows")
	}
	result := make([]float32, valueCount)
	for element := 0; element < accessor.Count; element++ {
		base := start + element*stride
		for component := 0; component < componentCount; component++ {
			offset := base + component*componentSize
			value, err := readFloatComponent(reader.bin[offset:offset+componentSize], accessor.ComponentType, accessor.Normalized)
			if err != nil {
				return nil, err
			}
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return nil, fmt.Errorf("accessor contains a non-finite value")
			}
			result[element*componentCount+component] = value
		}
	}
	return result, nil
}

func (reader accessorReader) layout(accessor *accessorDef) (bufferViewDef, int, int, int, error) {
	if accessor.BufferView == nil || *accessor.BufferView < 0 || *accessor.BufferView >= len(reader.document.BufferViews) {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor has an invalid bufferView")
	}
	view := reader.document.BufferViews[*accessor.BufferView]
	if view.Buffer != 0 {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("external or secondary buffers are not supported in GLB files")
	}
	if view.ByteOffset < 0 || view.ByteLength < 0 {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("bufferView has a negative offset or length")
	}
	viewEnd, ok := checkedAdd(view.ByteOffset, view.ByteLength)
	if !ok || viewEnd > len(reader.bin) {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("bufferView exceeds the BIN chunk")
	}

	componentCount, ok := typeComponentCount(accessor.Type)
	if !ok {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("unsupported accessor type %q", accessor.Type)
	}
	componentSize, ok := componentSize(accessor.ComponentType)
	if !ok {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("unsupported accessor component type %d", accessor.ComponentType)
	}
	elementSize, ok := checkedMultiply(componentCount, componentSize)
	if !ok {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor element size overflows")
	}
	stride := view.ByteStride
	if stride == 0 {
		stride = elementSize
	}
	if stride < elementSize || stride > 252 || stride%componentSize != 0 {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("bufferView has invalid byteStride %d", stride)
	}
	if accessor.ByteOffset%componentSize != 0 || view.ByteOffset%componentSize != 0 {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor data is not aligned to its component size")
	}

	lastOffset := accessor.ByteOffset
	if accessor.Count > 1 {
		span, ok := checkedMultiply(accessor.Count-1, stride)
		if !ok {
			return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor byte range overflows")
		}
		lastOffset, ok = checkedAdd(lastOffset, span)
		if !ok {
			return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor byte range overflows")
		}
	}
	endWithinView, ok := checkedAdd(lastOffset, elementSize)
	if !ok || endWithinView > view.ByteLength {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor exceeds its bufferView")
	}
	start, ok := checkedAdd(view.ByteOffset, accessor.ByteOffset)
	if !ok {
		return bufferViewDef{}, 0, 0, 0, fmt.Errorf("accessor start offset overflows")
	}
	return view, start, stride, componentCount, nil
}

func readFloatComponent(data []byte, componentType int, normalized bool) (float32, error) {
	switch componentType {
	case 5120:
		value := int8(data[0])
		if normalized {
			return max(float32(value)/127, -1), nil
		}
		return float32(value), nil
	case 5121:
		value := data[0]
		if normalized {
			return float32(value) / 255, nil
		}
		return float32(value), nil
	case 5122:
		value := int16(binary.LittleEndian.Uint16(data))
		if normalized {
			return max(float32(value)/32767, -1), nil
		}
		return float32(value), nil
	case 5123:
		value := binary.LittleEndian.Uint16(data)
		if normalized {
			return float32(value) / 65535, nil
		}
		return float32(value), nil
	case 5125:
		return float32(binary.LittleEndian.Uint32(data)), nil
	case 5126:
		return math.Float32frombits(binary.LittleEndian.Uint32(data)), nil
	default:
		return 0, fmt.Errorf("unsupported accessor component type %d", componentType)
	}
}

func typeComponentCount(accessorType string) (int, bool) {
	switch accessorType {
	case "SCALAR":
		return 1, true
	case "VEC2":
		return 2, true
	case "VEC3":
		return 3, true
	case "VEC4", "MAT2":
		return 4, true
	case "MAT3":
		return 9, true
	case "MAT4":
		return 16, true
	default:
		return 0, false
	}
}

func componentSize(componentType int) (int, bool) {
	switch componentType {
	case 5120, 5121:
		return 1, true
	case 5122, 5123:
		return 2, true
	case 5125, 5126:
		return 4, true
	default:
		return 0, false
	}
}

func checkedAdd(a, b int) (int, bool) {
	if a < 0 || b < 0 || a > int(^uint(0)>>1)-b {
		return 0, false
	}
	return a + b, true
}

func checkedMultiply(a, b int) (int, bool) {
	if a < 0 || b < 0 || (a != 0 && b > int(^uint(0)>>1)/a) {
		return 0, false
	}
	return a * b, true
}
