const positions = new Float32Array([
  -1, -1, 1, 1, -1, 1, 1, 1, 1, -1, 1, 1,
  1, -1, -1, -1, -1, -1, -1, 1, -1, 1, 1, -1,
  -1, 1, 1, 1, 1, 1, 1, 1, -1, -1, 1, -1,
  -1, -1, -1, 1, -1, -1, 1, -1, 1, -1, -1, 1,
  1, -1, 1, 1, -1, -1, 1, 1, -1, 1, 1, 1,
  -1, -1, -1, -1, -1, 1, -1, 1, 1, -1, 1, -1,
]);

const normals = new Float32Array([
  0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1,
  0, 0, -1, 0, 0, -1, 0, 0, -1, 0, 0, -1,
  0, 1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0,
  0, -1, 0, 0, -1, 0, 0, -1, 0, 0, -1, 0,
  1, 0, 0, 1, 0, 0, 1, 0, 0, 1, 0, 0,
  -1, 0, 0, -1, 0, 0, -1, 0, 0, -1, 0, 0,
]);

const uvs = new Float32Array([
  0, 1, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0,
  0, 1, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0,
  0, 1, 1, 1, 1, 0, 0, 0, 0, 1, 1, 1, 1, 0, 0, 0,
]);

const indices = new Uint16Array([
  0, 1, 2, 0, 2, 3, 4, 5, 6, 4, 6, 7,
  8, 9, 10, 8, 10, 11, 12, 13, 14, 12, 14, 15,
  16, 17, 18, 16, 18, 19, 20, 21, 22, 20, 22, 23,
]);

export function createDemoModel(): Uint8Array {
  const sources = [
    new Uint8Array(positions.buffer),
    new Uint8Array(normals.buffer),
    new Uint8Array(uvs.buffer),
    new Uint8Array(indices.buffer),
  ];
  const offsets: number[] = [];
  let binaryLength = 0;
  for (const source of sources) {
    offsets.push(binaryLength);
    binaryLength += source.length;
  }
  const binary = new Uint8Array(binaryLength);
  sources.forEach((source, index) => binary.set(source, offsets[index]));

  const document = {
    asset: { version: "2.0", generator: "Go WASM Model Viewer" },
    scene: 0,
    scenes: [{ nodes: [0] }],
    nodes: [{ mesh: 0 }],
    meshes: [{
      primitives: [{
        attributes: { POSITION: 0, NORMAL: 1, TEXCOORD_0: 2 },
        indices: 3,
        material: 0,
      }],
    }],
    materials: [{
      pbrMetallicRoughness: { baseColorFactor: [0.12, 0.48, 0.95, 1] },
    }],
    buffers: [{ byteLength: binaryLength }],
    bufferViews: sources.map((source, index) => ({
      buffer: 0,
      byteOffset: offsets[index],
      byteLength: source.length,
    })),
    accessors: [
      { bufferView: 0, componentType: 5126, count: 24, type: "VEC3" },
      { bufferView: 1, componentType: 5126, count: 24, type: "VEC3" },
      { bufferView: 2, componentType: 5126, count: 24, type: "VEC2" },
      { bufferView: 3, componentType: 5123, count: 36, type: "SCALAR" },
    ],
  };

  const encodedJSON = new TextEncoder().encode(JSON.stringify(document));
  const jsonLength = align4(encodedJSON.length);
  const paddedBinaryLength = align4(binary.length);
  const totalLength = 12 + 8 + jsonLength + 8 + paddedBinaryLength;
  const result = new Uint8Array(totalLength);
  const view = new DataView(result.buffer);
  view.setUint32(0, 0x46546c67, true);
  view.setUint32(4, 2, true);
  view.setUint32(8, totalLength, true);
  view.setUint32(12, jsonLength, true);
  view.setUint32(16, 0x4e4f534a, true);
  result.fill(0x20, 20, 20 + jsonLength);
  result.set(encodedJSON, 20);
  const binaryHeader = 20 + jsonLength;
  view.setUint32(binaryHeader, paddedBinaryLength, true);
  view.setUint32(binaryHeader + 4, 0x004e4942, true);
  result.set(binary, binaryHeader + 8);
  return result;
}

function align4(value: number): number {
  return (value + 3) & ~3;
}
